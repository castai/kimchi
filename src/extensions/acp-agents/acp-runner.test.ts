import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { setActiveManagerForTest } from "../agents/index.js"
import { AgentManager } from "../agents/manager/agent-manager.js"
import type { AgentRecord } from "../agents/personas/types.js"
import { runAcpAgent } from "./acp-runner.js"

const fixturePath = new URL("./test-fixtures/fake-acp-agent.mjs", import.meta.url).pathname

function makeRecord(server: string): AgentRecord {
	return {
		id: "acp-test-record",
		type: `acp:${server}`,
		description: "test ACP agent",
		visibility: "user",
		status: "running",
		acp: { server },
		toolUses: 0,
		startedAt: Date.now(),
		currentAttemptId: 0,
		lifetimeUsage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
		compactionCount: 0,
	}
}

describe("runAcpAgent", () => {
	let logDir: string
	let projectDir: string
	let globalDir: string
	const prevAgentDir = process.env.PI_CODING_AGENT_DIR

	beforeEach(() => {
		logDir = mkdtempSync(join(tmpdir(), "acp-runner-log-"))
		projectDir = mkdtempSync(join(tmpdir(), "acp-runner-project-"))
		globalDir = mkdtempSync(join(tmpdir(), "acp-runner-global-"))
		// Keep the real global config out of the test.
		process.env.PI_CODING_AGENT_DIR = globalDir

		mkdirSync(join(projectDir, ".kimchi"), { recursive: true })
		writeFileSync(
			join(projectDir, ".kimchi", "acp-agents.json"),
			JSON.stringify({
				agent_servers: {
					fake: {
						command: process.execPath,
						args: [fixturePath],
						env: { FAKE_ACP_LOG: join(logDir, "log.jsonl") },
					},
				},
			}),
		)
	})

	afterEach(() => {
		rmSync(logDir, { recursive: true, force: true })
		rmSync(projectDir, { recursive: true, force: true })
		rmSync(globalDir, { recursive: true, force: true })
		if (prevAgentDir === undefined) delete process.env.PI_CODING_AGENT_DIR
		else process.env.PI_CODING_AGENT_DIR = prevAgentDir
	})

	function makeCtx(): ExtensionContext {
		return { cwd: projectDir } as unknown as ExtensionContext
	}

	it("runs a prompt end-to-end and maps the RunResult", async () => {
		const record = makeRecord("fake")
		const onTextDelta = vi.fn()
		const onTurnEnd = vi.fn()

		const result = await runAcpAgent(record, "hello", { description: "test", onTextDelta, onTurnEnd }, makeCtx())

		expect(result.session).toBeUndefined()
		expect(result.responseText).toBe("echo:hello")
		expect(result.aborted).toBe(false)
		expect(result.abortReason).toBeUndefined()
		expect(result.steered).toBe(false)
		expect(result.turnsUsed).toBe(1)
		expect(onTextDelta).toHaveBeenCalledWith("echo:hello", "echo:hello")
		expect(onTurnEnd).toHaveBeenCalledWith(1)
		expect(record.lifetimeUsage).toEqual({ input: 11, output: 7, cacheRead: 0, cacheWrite: 0 })
		expect(record.lastTurnCount).toBe(1)
	})

	it("counts completed tool calls into the record", async () => {
		const record = makeRecord("fake")

		await runAcpAgent(record, "USE_TOOLS now", { description: "test" }, makeCtx())

		expect(record.toolUses).toBe(1)
	})

	it("throws for a server that is no longer configured", async () => {
		const record = makeRecord("missing")

		await expect(runAcpAgent(record, "hello", { description: "test" }, makeCtx())).rejects.toThrow(
			/no longer configured/,
		)
	})

	it("throws when the record has no ACP server", async () => {
		const record = makeRecord("fake")
		record.acp = undefined

		await expect(runAcpAgent(record, "hello", { description: "test" }, makeCtx())).rejects.toThrow(
			/requires record\.acp\.server/,
		)
	})

	it("enforces maxDuration by cancelling and reporting aborted", async () => {
		const record = makeRecord("fake")

		const result = await runAcpAgent(record, "BLOCK forever", { description: "test", maxDuration: 1 }, makeCtx())

		expect(result.aborted).toBe(true)
		expect(result.abortReason).toBe("max_duration")
	}, 15_000)

	it("forwards spawn callbacks for usage and text", async () => {
		const record = makeRecord("fake")
		const onAssistantUsage = vi.fn()

		await runAcpAgent(record, "hello", { description: "test", onAssistantUsage }, makeCtx())

		expect(onAssistantUsage).toHaveBeenCalledWith({ input: 11, output: 7, cacheRead: 0, cacheWrite: 0 })
	})

	it("aborts a running ACP agent through the manager and reaches a terminal state quickly", async () => {
		const manager = new AgentManager(undefined, 4)
		setActiveManagerForTest(manager)
		try {
			manager.setAcpRunner(runAcpAgent)
			const id = manager.spawn({} as ExtensionAPI, makeCtx(), "acp:fake", "BLOCK forever", {
				description: "abort target",
				isBackground: true,
				bypassQueue: true,
				acp: { server: "fake" },
			})
			const record = manager.getRecord(id)
			if (!record?.promise) throw new Error("expected a running ACP record")

			const started = Date.now()
			setTimeout(() => manager.abort(id), 1500)
			await record.promise

			// The abort must beat the fixture's 25s WAIT turn, not wait it out.
			expect(Date.now() - started).toBeLessThan(15_000)
			expect(["stopped", "aborted"]).toContain(record.status)
		} finally {
			await manager.waitForAll().catch(() => {})
			manager.dispose()
			setActiveManagerForTest(undefined)
		}
	}, 30_000)

	it("delivers queued steers as a follow-up turn", async () => {
		const manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		try {
			const id = manager.spawn({} as ExtensionAPI, makeCtx(), "Explore", "acp run", {
				description: "acp run",
				isBackground: true,
			})
			const record = manager.getRecord(id)
			if (!record) throw new Error("expected a queued record")
			record.acp = { server: "fake" }
			record.pendingSteers = ["steer one", "steer two"]

			const result = await runAcpAgent(record, "hello", { description: "test" }, makeCtx())

			// Turn 1 = the task prompt; turn 2 = the queued steers (combined).
			expect(result.turnsUsed).toBe(2)
			expect(result.responseText).toBe("echo:steer one\n\nsteer two")
			expect(record.pendingSteers).toBeUndefined()
		} finally {
			manager.dispose()
			setActiveManagerForTest(undefined)
		}
	})

	it("delivers pending peer messages as a follow-up turn", async () => {
		const manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		try {
			manager.bindCommunicationRoot("root-1")
			manager.registerParentBridge("root-1", () => true)
			const sourceId = manager.spawn({} as ExtensionAPI, makeCtx(), "Explore", "source", {
				description: "source",
				isBackground: true,
				communication: "group",
				rootSessionId: "root-1",
			})
			const targetId = manager.spawn({} as ExtensionAPI, makeCtx(), "Explore", "target", {
				description: "target",
				isBackground: true,
				communication: "group",
				rootSessionId: "root-1",
			})
			const source = manager.getRecord(sourceId)
			const target = manager.getRecord(targetId)
			if (!source || !target) throw new Error("expected records")
			source.groupId = "batch-1"
			target.groupId = "batch-1"
			target.acp = { server: "fake" }

			// The source asks the ACP target a question; the target has no
			// session, so the broker queues it (queued_before_session).
			const capability = manager.getAgentCommsCapability(sourceId)
			if (!capability) throw new Error("expected comms capability")
			const receipt = await capability.sendMessage("t-1", {
				recipient: { type: "agent", agentId: targetId },
				payload: { kind: "question", question: "peer question", impact: "correctness", canContinue: false },
			})
			expect(receipt).toMatchObject({ status: "queued_before_session" })

			const result = await runAcpAgent(target, "hello", { description: "test" }, makeCtx())

			expect(result.turnsUsed).toBe(2)
			expect(result.responseText).toContain("echo:Host-mediated message from peer")
			// The pending delivery was consumed by the drain.
			expect(manager.getMessageBrokerStats().pendingMessages).toBe(0)
		} finally {
			manager.dispose()
			setActiveManagerForTest(undefined)
		}
	})
})
