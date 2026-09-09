/**
 * fake-acp-agent.mjs — minimal ACP agent over stdio for tests.
 *
 * Speaks just enough JSON-RPC 2.0 / ACP for StdioAcpClient tests:
 * initialize, newSession, prompt, session/cancel, session/update
 * notifications, and session/request_permission.
 *
 * Behavior is driven by the prompt text:
 *   - "USE_TOOLS ..."  → emits tool_call start+completed before responding
 *   - "PERM ..."       → sends a permission request, logs the client's
 *                        outcome, then finishes the turn
 *   - "BLOCK ..."      → does not answer the prompt until session/cancel
 *   - "EXIT_DURING ..."→ exits immediately (child death mid-turn)
 *   - otherwise        → streams `echo:<text>` as agent_message_chunk
 *
 * When FAKE_ACP_LOG is set, startup, received newSession params, permission
 * outcomes, the setModel request, a blocked prompt, and process exit are
 * appended as JSON lines to that file so tests can assert on what the
 * client actually sent. When FAKE_ACP_MODE=hang, initialize never responds
 * (timeout testing).
 */

import { appendFileSync } from "node:fs"

const logPath = process.env.FAKE_ACP_LOG
function log(entry) {
	if (!logPath) return
	try {
		appendFileSync(logPath, `${JSON.stringify(entry)}\n`)
	} catch {
		/* best effort */
	}
}

let buffer = ""
let nextRequestId = 1
let pendingCancelPromptId = null
let pendingPermPromptId = null

function send(obj) {
	process.stdout.write(`${JSON.stringify(obj)}\n`)
}

function respond(id, result) {
	send({ jsonrpc: "2.0", id, result })
}

function notify(sessionId, update) {
	send({ jsonrpc: "2.0", method: "session/update", params: { sessionId, update } })
}

process.stdin.setEncoding("utf8")
process.stdin.on("data", (chunk) => {
	buffer += chunk
	const lines = buffer.split("\n")
	buffer = lines.pop() ?? ""
	for (const raw of lines) {
		const line = raw.trim()
		if (line) handle(JSON.parse(line))
	}
})

process.stdin.on("end", () => {
	log({ type: "exit" })
	process.exit(0)
})

function handle(msg) {
	// JSON-RPC responses (no method) are replies to requests we sent —
	// i.e. permission outcomes.
	if (msg.method === undefined) {
		log({ type: "permissionOutcome", result: msg.result })
		if (pendingPermPromptId !== null) {
			respond(pendingPermPromptId, { stopReason: "end_turn" })
			pendingPermPromptId = null
		}
		return
	}

	switch (msg.method) {
		case "initialize": {
			if (process.env.FAKE_ACP_MODE === "hang") return // never respond
			respond(msg.id, { protocolVersion: msg.params?.protocolVersion ?? "1", agentCapabilities: {} })
			break
		}
		case "session/new": {
			log({ type: "newSession", params: msg.params })
			respond(msg.id, { sessionId: "fake-session-1" })
			break
		}
		case "session/prompt": {
			const sessionId = msg.params.sessionId
			const text = msg.params.prompt?.[0]?.text ?? ""
			if (text.startsWith("BLOCK")) {
				log({ type: "blockStarted" })
				pendingCancelPromptId = msg.id
				return
			}
			if (text.startsWith("EXIT_DURING")) {
				process.exit(3)
			}
			if (text.startsWith("PERM")) {
				pendingPermPromptId = msg.id
				send({
					jsonrpc: "2.0",
					id: nextRequestId++,
					method: "session/request_permission",
					params: {
						sessionId,
						toolCall: { toolCallId: "perm-1", title: "needs permission", kind: "execute" },
						options: [{ optionId: "allow", name: "Allow", kind: "allow_once" }],
					},
				})
				return
			}
			if (text.startsWith("USE_TOOLS")) {
				notify(sessionId, { sessionUpdate: "tool_call", toolCallId: "tc1", title: "fake tool", status: "in_progress" })
				notify(sessionId, { sessionUpdate: "tool_call", toolCallId: "tc1", title: "fake tool", status: "completed" })
			}
			notify(sessionId, {
				sessionUpdate: "agent_message_chunk",
				content: { type: "text", text: `echo:${text}` },
			})
			respond(msg.id, {
				stopReason: "end_turn",
				usage: { inputTokens: 11, outputTokens: 7, cachedReadTokens: 0, cachedWriteTokens: 0 },
			})
			break
		}
		case "session/set_model": {
			log({ type: "setModel", model: msg.params?.modelId ?? msg.params?.model })
			respond(msg.id, {})
			break
		}
		case "session/cancel": {
			// session/cancel arrives as a JSON-RPC notification (no id) — only
			// respond when it carries one, else the response orphans.
			if (msg.id !== undefined) respond(msg.id, {})
			if (pendingCancelPromptId !== null) {
				respond(pendingCancelPromptId, { stopReason: "cancelled" })
				pendingCancelPromptId = null
			}
			break
		}
		default: {
			// Unknown requests get an empty object response.
			if (msg.id !== undefined) respond(msg.id, {})
		}
	}
}

log({ type: "started", pid: process.pid })
