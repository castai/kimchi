/**
 * acp-runner.ts — runs an ACP external agent as a subagent-plane node.
 *
 * Mirrors `_runRemote()` in AgentManager: resolves the server config, drives
 * an ACP client (stdio child process, or WebSocket reusing the sandbox
 * client) through the turn loop, maps `AcpSessionCallbacks` onto the spawn
 * callbacks, and returns a `RemoteRunResult`. One `prompt()` call is one
 * turn; steers and pending peer messages are delivered as follow-up prompts
 * between turns (see the drain hook in the loop).
 *
 * v1 limitations (documented in docs/acp-agents.md):
 * - `steered` stays false: ACP records never enter the in-process `steered`
 *   terminal status; steer-equivalents are delivered as prompts and the
 *   record completes normally.
 * - Transcript files for ACP agents hold the initial entry and the final
 *   result only — there is no in-process session to subscribe to.
 * - WS transport is deny-permissions-only (the sandbox client rejects all
 *   permission requests unconditionally).
 */

import type { ExtensionContext } from "@earendil-works/pi-coding-agent"
import { type AcpPromptResult, type AcpSessionCallbacks, AcpSessionClient } from "../../sandbox/worker/acp-client.js"
import { getAgentInvocation } from "../../utils/spawn-kimchi-subprocess.js"
import { getActiveManager } from "../agents/index.js"
import type { PendingAgentMessage, RemoteRunResult, SpawnOptions } from "../agents/manager/agent-manager.js"
import { addUsage, type LifetimeUsage } from "../agents/manager/usage.js"
import type { AgentAbortReason, AgentRecord } from "../agents/personas/types.js"
import { type AcpMcpServer, StdioAcpClient } from "./acp-agent-client.js"
import { getAgentCommsIpc } from "./comms-ipc.js"
import { type AcpAgentServerConfig, loadAcpAgentServers } from "./config.js"

/** The client surface both transports expose to the runner. */
interface AcpClientSurface {
	initialize(): Promise<void>
	prompt(text: string): Promise<AcpPromptResult>
	cancel(): Promise<void>
	close(): void
}

/** Far-future expiry for synthesized WS credentials (the field is required but unused by the client). */
const SYNTHETIC_EXPIRES_AT = "2999-01-01T00:00:00.000Z"

function hostFromUrl(url: string): string {
	try {
		return new URL(url).host
	} catch {
		return "unknown"
	}
}

function buildAcpClient(
	config: AcpAgentServerConfig,
	record: AgentRecord,
	ctx: ExtensionContext,
	callbacks: AcpSessionCallbacks,
	mcpServers: AcpMcpServer[],
): AcpClientSurface {
	if (config.transport === "stdio") {
		if (!config.command) throw new Error(`ACP server "${config.name}" is missing its command.`)
		return new StdioAcpClient({
			command: config.command,
			args: config.args,
			env: config.env,
			cwd: config.cwd ?? ctx.cwd,
			callbacks,
			signal: record.abortController?.signal,
			mcpServers,
			permissions: config.permissions,
		})
	}
	if (!config.url) throw new Error(`ACP server "${config.name}" is missing its url.`)
	return new AcpSessionClient({
		sessionName: config.sessionName ?? `acp-${record.id.slice(0, 8)}`,
		credentials: {
			wsUrl: config.url,
			connectToken: config.token ?? "",
			host: hostFromUrl(config.url),
			expiresAt: SYNTHETIC_EXPIRES_AT,
		},
		cwd: ctx.cwd,
		callbacks,
		signal: record.abortController?.signal,
		mcpServers,
	})
}

/**
 * Runs one ACP external agent to completion. Called by AgentManager._runAcp
 * via the setAcpRunner injection (acp-agents extension).
 */
export async function runAcpAgent(
	record: AgentRecord,
	prompt: string,
	options: SpawnOptions,
	ctx: ExtensionContext,
): Promise<RemoteRunResult> {
	const serverName = record.acp?.server
	if (!serverName) throw new Error("ACP run requires record.acp.server.")
	const config = loadAcpAgentServers(ctx.cwd).get(serverName)
	if (!config) {
		throw new Error(
			`ACP agent server "${serverName}" is no longer configured (checked .kimchi/acp-agents.json and the global config).`,
		)
	}

	// MCP servers handed to the external agent in newSession. When
	// communication is enabled, the host comms shim (board + messaging) is
	// passed so the external agent can call the same tools as in-process
	// peers — through host-authorized IPC, never by trusting model input.
	// Token revocation is manager-owned (transitionToTerminalRecord).
	const mcpServers: AcpMcpServer[] = []
	if (record.communication && record.communicationScope) {
		const ipc = getAgentCommsIpc()
		const socketPath = ipc.ensureStarted()
		const token = ipc.registerToken(record.id)
		mcpServers.push(getAgentInvocation(["--agent-comms-mcp", socketPath, token]))
	}

	let turnText = ""
	let responseText = ""
	const callbacks: AcpSessionCallbacks = {
		onTextDelta: (delta, fullText) => {
			turnText = fullText
			options.onTextDelta?.(delta, fullText)
		},
		onToolActivity: (activity) => {
			if (activity.type === "end") record.toolUses++
			options.onToolActivity?.(activity)
		},
		onTurnEnd: (turnCount) => {
			record.lastTurnCount = turnCount
			options.onTurnEnd?.(turnCount)
		},
		onAssistantUsage: (usage: LifetimeUsage) => {
			addUsage(record.lifetimeUsage, usage)
			options.onAssistantUsage?.(usage)
		},
		onRawNotification: (params) => options.onRawNotification?.(params),
	}

	const client = buildAcpClient(config, record, ctx, callbacks, mcpServers)
	let abortReason: AgentAbortReason | undefined
	let durationTimer: ReturnType<typeof setTimeout> | undefined

	try {
		await client.initialize()

		// maxDuration enforcement (seconds): cancel the in-flight turn; the
		// client's prompt stall timeout is the backstop for agents that ignore
		// session/cancel.
		const maxDuration = options.maxDuration
		if (maxDuration != null && maxDuration > 0) {
			durationTimer = setTimeout(() => {
				abortReason = "max_duration"
				client.cancel().catch(() => {})
			}, maxDuration * 1000)
			durationTimer.unref?.()
		}

		let turnsUsed = 0
		let cancelled = false

		// Turn loop: the initial prompt, then steers and pending peer messages
		// delivered as follow-up prompts through the manager's ACP drain —
		// steers first (orchestrator directives, combined), then broker
		// messages. Follow-ups are only taken while the turn budget allows
		// another turn; undelivered work stays queued and is terminalized by the
		// record's terminal transition (never silently dropped).
		let next: { prompt: string; pending?: PendingAgentMessage } | undefined = { prompt }
		while (next !== undefined) {
			const result = await client.prompt(next.prompt)
			turnsUsed++
			if (result.stopReason === "cancelled") cancelled = true
			if (turnText.trim()) responseText = turnText
			turnText = ""
			const manager = getActiveManager()
			manager?.completeAcpFollowUp(next.pending)
			next = options.maxTurns != null && turnsUsed >= options.maxTurns ? undefined : manager?.takeAcpFollowUp(record.id)
		}

		return {
			responseText,
			session: undefined,
			aborted: abortReason !== undefined || cancelled,
			abortReason,
			steered: false,
			turnsUsed,
			maxTurns: options.maxTurns,
		}
	} finally {
		if (durationTimer) clearTimeout(durationTimer)
		client.close()
	}
}
