/**
 * Session-level regression tests for the mid-turn compaction guard coordination.
 *
 * Reproduces the double-compaction incident from session 01a06c70 (2026-09-07):
 * the model-guard turn_end guard fired ctx.compact() (upstream's aborting manual
 * path) while the context was over the compaction threshold. compact() aborted
 * the in-flight run (the run's next LLM call died with "The operation was
 * aborted." and a queued steering message was lost), the run ended with
 * stop=error, and the post-run _checkCompaction then fired a SECOND (threshold)
 * compaction that raced the first. The manual attempt, parked on the session
 * idle promise, woke up after the auto compaction had appended its entry and
 * failed with "Already compacted" — the user saw two "Compacting…" phases.
 *
 * The fix makes the guard prefer ctx.inlineCompact (non-aborting). These tests
 * wire the REAL model-guard extension handlers to a REAL AgentSession (fake
 * model + scripted streamFn) and assert the session-level outcome:
 *   - Scenario A: exactly one compaction, the run continues on the compacted
 *     context, no abort error, no duplicate post-run compaction.
 *   - Scenario B: when the inline compaction fails, the post-run threshold
 *     compaction still recovers — exactly one compaction entry overall.
 *
 * Scenario A fails against the pre-fix guard (double compaction_start, abort
 * error, uncompacted context for the next call).
 */
import { mkdtempSync } from "node:fs"
import os from "node:os"
import path from "node:path"
import { type AssistantMessage, createAssistantMessageEventStream, type Model } from "@earendil-works/pi-ai"
import {
	AgentSession,
	DefaultResourceLoader,
	type ExtensionAPI,
	type ExtensionContext,
	type ExtensionRunner,
	type ModelRuntime,
	SessionManager,
	SettingsManager,
} from "@earendil-works/pi-coding-agent"
import { beforeEach, describe, expect, it, type MockInstance, vi } from "vitest"
import {
	Agent,
	type AgentMessage,
} from "../../node_modules/.pnpm/node_modules/@earendil-works/pi-agent-core/dist/index.js"
import { getCompactionEnabled } from "../settings-watcher.js"
import { type InlineCompactOptions, installInlineCompactPatch } from "../upstream-inline-compact-patch.js"
import modelGuardExtension from "./model-guard.js"

vi.mock("../settings-watcher.js", () => ({
	getCompactionEnabled: vi.fn(() => true),
}))

installInlineCompactPatch()

// Production incident values: kimi-k3 (262,144 window), last toolUse response at
// 246,245 tokens — just over the 245,760 compaction threshold.
const CONTEXT_WINDOW = 262_144
const OVER_THRESHOLD_TOKENS = 246_245

function makeModel(): Model<"openai-completions"> {
	return {
		api: "openai-completions",
		provider: "kimchi-dev",
		id: "kimi-k3",
		name: "kimi-k3",
		reasoning: false,
		input: ["text"],
		contextWindow: CONTEXT_WINDOW,
		maxTokens: 60_000,
		cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
	} as unknown as Model<"openai-completions">
}

function makeAssistantMessage(extra: Partial<AssistantMessage>): AssistantMessage {
	return {
		role: "assistant",
		content: [],
		api: "openai-completions",
		provider: "kimchi-dev",
		model: "kimi-k3",
		usage: {
			input: 0,
			output: 0,
			cacheRead: 0,
			cacheWrite: 0,
			totalTokens: 0,
			cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
		},
		stopReason: "stop",
		timestamp: Date.now(),
		...extra,
	} as AssistantMessage
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

interface ScenarioOptions {
	/** Make the first summarizer call fail (inline compaction fails mid-run). */
	failFirstSummarizer: boolean
	/** Usage reported by the run's second main-loop response. */
	secondCallTokens: number
}

interface ScenarioResult {
	compactionStarts: number
	compactionEndFailures: string[]
	compactionEntries: number
	/** Contexts received by main-loop LLM calls after the first one. */
	subsequentCallContexts: Array<Array<{ role: string }>>
	/** Assistant messages persisted with stop=error. */
	errorMessages: string[]
	notify: ReturnType<typeof vi.fn>
	warnCalls: Array<unknown[]>
}

async function runScenario(options: ScenarioOptions): Promise<ScenarioResult> {
	const tmp = mkdtempSync(path.join(os.tmpdir(), "kimchi-compaction-coord-"))
	const model = makeModel()
	const settingsManager = SettingsManager.create(tmp, tmp)
	const loader = new DefaultResourceLoader({ cwd: tmp, agentDir: tmp, settingsManager })
	await loader.reload()

	let mainLoopCalls = 0
	let summarizerCalls = 0
	const subsequentCallContexts: Array<Array<{ role: string }>> = []

	const streamFn = (
		_m: unknown,
		context: { systemPrompt?: string; messages: unknown[] },
		callOptions?: { signal?: AbortSignal },
	) => {
		const stream = createAssistantMessageEventStream()
		if (String(context.systemPrompt ?? "").includes("summariz")) {
			summarizerCalls++
			const call = summarizerCalls
			queueMicrotask(async () => {
				// Production-like summarization latency: long enough that the run's
				// next LLM call completes before the compaction entry is appended
				// (the exact interleaving that produced the incident).
				await sleep(50)
				if (call === 1 && options.failFirstSummarizer) {
					stream.push({
						type: "error",
						reason: "error",
						error: makeAssistantMessage({ stopReason: "error", errorMessage: "summariser exploded" }),
					})
					return
				}
				stream.push({
					type: "done",
					reason: "stop",
					message: makeAssistantMessage({
						content: [{ type: "text", text: `Summary #${call} of prior conversation.` }],
					}),
				})
			})
			return stream
		}

		mainLoopCalls++
		if (mainLoopCalls === 1) {
			// Over-threshold toolUse response that arms the mid-turn guard.
			queueMicrotask(() =>
				stream.push({
					type: "done",
					reason: "toolUse",
					message: makeAssistantMessage({
						content: [{ type: "toolCall", id: "call-1", name: "bash", arguments: { command: "echo hi" } }],
						stopReason: "toolUse",
						usage: {
							...makeAssistantMessage({}).usage,
							input: 1_982,
							output: 679,
							cacheRead: 243_584,
							totalTokens: OVER_THRESHOLD_TOKENS,
						},
					}),
				}),
			)
			return stream
		}

		// Subsequent main-loop calls: honour the abort signal exactly like the
		// openai-completions wrapper does when the run is aborted mid-flight.
		subsequentCallContexts.push(context.messages as Array<{ role: string }>)
		if (callOptions?.signal?.aborted) {
			stream.push({
				type: "error",
				reason: "error",
				error: makeAssistantMessage({ stopReason: "error", errorMessage: "The operation was aborted." }),
			})
			return stream
		}
		queueMicrotask(() =>
			stream.push({
				type: "done",
				reason: "stop",
				message: makeAssistantMessage({
					content: [{ type: "text", text: "done" }],
					usage: {
						...makeAssistantMessage({}).usage,
						input: options.secondCallTokens,
						totalTokens: options.secondCallTokens,
					},
				}),
			}),
		)
		return stream
	}

	const extensionRunnerRef: { current?: ExtensionRunner } = {}
	const agent = new Agent({
		initialState: { systemPrompt: "", model, thinkingLevel: "off", tools: [] },
		convertToLlm: (messages: unknown[]) => messages,
		// Mirrors sdk.js: route context building through the extension runner so
		// the inline-compact resync patch (emitContext substitution) engages.
		transformContext: async (messages: AgentMessage[]) => extensionRunnerRef.current?.emitContext(messages) ?? messages,
		streamFn: (
			m: unknown,
			context: { systemPrompt?: string; messages: unknown[] },
			callOptions?: { signal?: AbortSignal },
		) => streamFn(m, context, callOptions),
	} as ConstructorParameters<typeof Agent>[0])

	const session = new AgentSession({
		agent,
		sessionManager: SessionManager.inMemory(tmp),
		settingsManager,
		cwd: tmp,
		modelRuntime: {
			hasConfiguredAuth: () => true,
			checkAuth: async () => undefined,
			getAuth: async () => undefined,
			isUsingOAuth: () => false,
			getModel: () => model,
		} as unknown as ModelRuntime,
		resourceLoader: loader,
		extensionRunnerRef,
	})

	// Seed enough history that prepareCompaction finds a cut point above the
	// default keepRecentTokens (20k): 40 pairs x ~4k chars ≈ ~40k estimated tokens.
	const filler = "x".repeat(4000)
	for (let i = 0; i < 40; i++) {
		session.sessionManager.appendMessage({
			role: "user",
			content: [{ type: "text", text: `history question ${i}: ${filler}` }],
			timestamp: Date.now(),
		})
		session.sessionManager.appendMessage({
			role: "assistant",
			content: [{ type: "text", text: `history answer ${i}: ${filler}` }],
			api: "openai-completions",
			provider: "kimchi-dev",
			model: "kimi-k3",
			usage: makeAssistantMessage({}).usage,
			stopReason: "stop",
			timestamp: Date.now(),
		})
	}
	agent.state.messages = [...session.sessionManager.buildSessionContext().messages]

	// Wire the REAL model-guard extension handlers to the real session.
	const handlers = new Map<string, (event: unknown, ctx: ExtensionContext) => Promise<unknown>>()
	modelGuardExtension({
		on: (event: string, handler: (e: unknown, ctx: ExtensionContext) => Promise<unknown>) => {
			handlers.set(event, handler)
		},
		registerCommand: () => {},
	} as unknown as ExtensionAPI)

	const notify = vi.fn()
	const sessionWithInline = session as unknown as {
		inlineCompact?: (options?: InlineCompactOptions) => Promise<{ tokensBefore?: number }>
		compact: (customInstructions?: string, force?: boolean) => Promise<unknown>
	}
	const ctx = {
		model,
		getContextUsage: () => session.getContextUsage(),
		sessionManager: session.sessionManager,
		// Mirrors the real createContext wiring (installInlineCompactPatch).
		inlineCompact: (inlineOptions?: InlineCompactOptions) => sessionWithInline.inlineCompact?.(inlineOptions),
		// Mirrors the real createContext wiring for the legacy fallback path.
		compact: (compactOptions?: { customInstructions?: string; force?: boolean; onError?: (e: Error) => void }) => {
			void sessionWithInline.compact(compactOptions?.customInstructions, compactOptions?.force ?? false).then(
				() => {},
				(error: Error) => compactOptions?.onError?.(error),
			)
		},
		ui: { notify },
	} as unknown as ExtensionContext

	const result: ScenarioResult = {
		compactionStarts: 0,
		compactionEndFailures: [],
		compactionEntries: 0,
		subsequentCallContexts,
		errorMessages: [],
		notify,
		warnCalls: [],
	}

	session.subscribe((event) => {
		if (event.type === "compaction_start") result.compactionStarts++
		if (event.type === "compaction_end" && !event.result && event.errorMessage) {
			result.compactionEndFailures.push(event.errorMessage)
		}
	})

	// The agent loop awaits subscriber listeners, exactly like it awaits the
	// extension runner's turn_end emit in production.
	void agent.subscribe(async (event: { type: string }) => {
		if (event.type === "turn_end") {
			await handlers.get("turn_end")?.(event, ctx)
		}
	})

	const warnSpy: MockInstance = vi.spyOn(console, "warn").mockImplementation((...args: unknown[]) => {
		result.warnCalls.push(args)
	})

	try {
		await session.prompt("start work")
		await sleep(200)

		const branch = session.sessionManager.getBranch()
		result.compactionEntries = branch.filter((e) => e.type === "compaction").length
		for (const entry of branch) {
			if (entry.type === "message" && entry.message.role === "assistant" && entry.message.stopReason === "error") {
				result.errorMessages.push(String(entry.message.errorMessage ?? ""))
			}
		}
	} finally {
		warnSpy.mockRestore()
	}

	return result
}

describe("mid-turn compaction guard coordination (session level)", () => {
	beforeEach(() => {
		vi.mocked(getCompactionEnabled).mockReturnValue(true)
	})

	it("Scenario A: compacts inline once, run continues on the compacted context, no duplicate", async () => {
		const result = await runScenario({ failFirstSummarizer: false, secondCallTokens: 500 })

		// Exactly one compaction attempt ran (the inline one) and one entry landed.
		expect(result.compactionStarts).toBe(1)
		expect(result.compactionEntries).toBe(1)
		expect(result.compactionEndFailures).toEqual([])

		// The run was not aborted by the compaction.
		expect(result.errorMessages).toEqual([])

		// The run's next LLM call received the compacted context (resync patch):
		// convertToLlm flattens the compactionSummary entry into a user message
		// carrying the summary text, and the seeded history is gone.
		expect(result.subsequentCallContexts.length).toBe(1)
		const secondCallContext = result.subsequentCallContexts[0]
		expect(secondCallContext.some((m) => JSON.stringify(m).includes("Summary #1 of prior conversation."))).toBe(true)
		expect(secondCallContext.length).toBeLessThan(60)

		// The user is informed about the transparent compaction.
		expect(result.notify).toHaveBeenCalledWith(expect.stringContaining("Context compacted"), "info")
	}, 30_000)

	it("Scenario B: when the inline compaction fails, the post-run threshold compaction recovers", async () => {
		const result = await runScenario({ failFirstSummarizer: true, secondCallTokens: OVER_THRESHOLD_TOKENS })

		// The inline attempt failed and was reported.
		const failedWarn = result.warnCalls.find(
			(args) =>
				String(args[0]).includes("mid-turn compaction failed") && String(args[1]).includes("summariser exploded"),
		)
		expect(failedWarn).toBeDefined()

		// The post-run threshold compaction recovered: exactly one entry overall,
		// one failed attempt, one successful compaction.
		expect(result.compactionEntries).toBe(1)
		expect(result.compactionStarts).toBe(2)
		expect(result.compactionEndFailures).toHaveLength(1)
		expect(result.compactionEndFailures[0]).toContain("summariser exploded")
	}, 30_000)
})
