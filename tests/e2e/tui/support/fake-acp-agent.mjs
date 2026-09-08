/**
 * fake-acp-agent.mjs — scripted ACP agent for the TUI e2e test.
 *
 * Speaks ACP over stdio. On session/new it spawns the MCP comms shim from the
 * mcpServers entry the host passed (proving the full external board path:
 * host IPC → board). On session/prompt it posts a finding to the coordination
 * board through that shim, streams a short user-visible line, and ends the
 * turn. The streamed text deliberately avoids the board entry title so the
 * e2e's coordinator-side match predicate can only fire on the board-update
 * follow-up, not on the agent result.
 */

import { spawn } from "node:child_process"

let stdinBuffer = ""
let shim = null
let shimStdout = ""
let mcpId = 0
const pendingMcp = new Map()
let shimReady = null

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
	stdinBuffer += chunk
	const lines = stdinBuffer.split("\n")
	stdinBuffer = lines.pop() ?? ""
	for (const raw of lines) {
		const line = raw.trim()
		if (line) handle(JSON.parse(line))
	}
})
process.stdin.on("end", () => {
	if (shim) shim.kill()
	process.exit(0)
})

function mcpRequest(method, params) {
	return new Promise((resolve, reject) => {
		const id = ++mcpId
		const timer = setTimeout(() => {
			pendingMcp.delete(id)
			reject(new Error(`MCP ${method} timed out`))
		}, 20_000)
		pendingMcp.set(id, {
			resolve: (res) => {
				clearTimeout(timer)
				resolve(res)
			},
		})
		shim.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`)
	})
}

function startShim(mcpServers) {
	const server = mcpServers?.[0]
	if (!server) return Promise.reject(new Error("expected an mcpServers entry with the host comms shim"))
	shim = spawn(server.command, server.args ?? [], { stdio: ["pipe", "pipe", "ignore"] })
	shim.stdout.setEncoding("utf8")
	shim.stdout.on("data", (chunk) => {
		shimStdout += chunk
		const lines = shimStdout.split("\n")
		shimStdout = lines.pop() ?? ""
		for (const raw of lines) {
			const line = raw.trim()
			if (!line) continue
			let res
			try {
				res = JSON.parse(line)
			} catch {
				continue
			}
			if (res.id !== undefined && pendingMcp.has(res.id)) {
				pendingMcp.get(res.id).resolve(res)
				pendingMcp.delete(res.id)
			}
		}
	})
	return mcpRequest("initialize", {
		protocolVersion: "2024-11-05",
		capabilities: {},
		clientInfo: { name: "fake-acp-agent", version: "1.0.0" },
	}).then(() => {
		shim.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" })}\n`)
	})
}

async function mcpCall(name, arguments_) {
	const res = await mcpRequest("tools/call", { name, arguments: arguments_ })
	return {
		text: res?.result?.content?.[0]?.text ?? "",
		isError: res?.result?.isError === true,
	}
}

async function postFindingToBoard() {
	const res = await mcpCall("post_agent_note", {
		kind: "finding",
		title: "ACP-EXTERNAL-POSTED",
		body: "Posted by the external ACP agent through the host comms shim.",
	})
	return res.text.includes('"ok":true') ? "yes" : `no (${res.text.slice(0, 120)})`
}

/** Asks the first live peer a question through the MCP comms shim. */
async function askPeerQuestion() {
	const list = await mcpCall("list_agent_contacts", {})
	let contacts
	try {
		contacts = JSON.parse(list.text)
	} catch {
		return `contacts unparsable (${list.text.slice(0, 160)})`
	}
	const peer = contacts.peers?.[0]
	if (!peer?.agent_id) return `no peer found (contacts: ${list.text.slice(0, 160)})`
	const res = await mcpCall("send_agent_message", {
		recipient: { type: "agent", agentId: peer.agent_id },
		payload: {
			kind: "question",
			question: "External-to-external check: please confirm you received this directly.",
			impact: "validation of external peer messaging",
			canContinue: true,
		},
	})
	return res.isError
		? `send FAILED (${res.text.slice(0, 160)})`
		: `asked peer ${peer.agent_id}: ${res.text.slice(0, 160)}`
}

/** Answers a host-mediated peer question through the MCP comms shim. */
async function answerPeerQuestion(sourceAgentId, messageId) {
	const res = await mcpCall("send_agent_message", {
		recipient: { type: "agent", agentId: sourceAgentId },
		payload: {
			kind: "answer",
			answer: "Confirmed: I am the external ACP agent, answering through the host comms shim.",
		},
		reply_to: messageId,
	})
	return res.isError ? `answer FAILED (${res.text.slice(0, 160)})` : res.text.slice(0, 160)
}

function handle(msg) {
	switch (msg.method) {
		case "initialize": {
			respond(msg.id, { protocolVersion: msg.params?.protocolVersion ?? "1", agentCapabilities: {} })
			break
		}
		case "session/new": {
			shimReady = startShim(msg.params?.mcpServers).catch((err) => {
				process.stderr.write(`shim failed: ${err.message}\n`)
			})
			respond(msg.id, { sessionId: "acp-fake-1" })
			break
		}
		case "session/prompt": {
			const sessionId = msg.params.sessionId
			const text = msg.params.prompt?.[0]?.text ?? ""

			// Peer question delivered as a follow-up turn (host-mediated):
			// answer it back through the MCP comms shim with reply_to.
			const peer = text.match(/Host-mediated message from peer ([0-9a-f-]+) message_id=([0-9a-f-]+):/)
			if (peer) {
				Promise.resolve(shimReady)
					.then(() => answerPeerQuestion(peer[1], peer[2]))
					.then((receipt) => {
						notify(sessionId, {
							sessionUpdate: "agent_message_chunk",
							content: {
								type: "text",
								text: `ACP-PEER-ANSWERED: replied to peer ${peer[1]} (reply_to ${peer[2]}): ${receipt}`,
							},
						})
						respond(msg.id, { stopReason: "end_turn" })
					})
					.catch((err) => {
						notify(sessionId, {
							sessionUpdate: "agent_message_chunk",
							content: { type: "text", text: `ACP-PEER-ANSWERED: failed (${err.message})` },
						})
						respond(msg.id, { stopReason: "end_turn" })
					})
				break
			}

			// ASK_PEER mode: discover a peer via MCP list_agent_contacts, ask it
			// one question, then stay busy ~30s so the peer can answer while this
			// agent is still live (the answer arrives as a follow-up turn and
			// echoes through the default branch below).
			if (text.startsWith("ASK_PEER")) {
				Promise.resolve(shimReady)
					.then(() => askPeerQuestion())
					.then(async (asked) => {
						await new Promise((r) => setTimeout(r, 30_000))
						return asked
					})
					.then((asked) => {
						notify(sessionId, {
							sessionUpdate: "agent_message_chunk",
							content: { type: "text", text: `ACP-EXTERNAL-ASKER: ${asked}` },
						})
						respond(msg.id, { stopReason: "end_turn" })
					})
					.catch((err) => {
						notify(sessionId, {
							sessionUpdate: "agent_message_chunk",
							content: { type: "text", text: `ACP-EXTERNAL-ASKER: failed (${err.message})` },
						})
						respond(msg.id, { stopReason: "end_turn" })
					})
				break
			}

			// WAIT mode: post to the board, then stay busy ~25s so live peers can
			// queue questions before this turn ends (they drain as follow-up turns).
			const slow = text.startsWith("WAIT")
			Promise.resolve(shimReady)
				.then(() => postFindingToBoard())
				.then(async (posted) => {
					if (slow) await new Promise((r) => setTimeout(r, 25_000))
					return posted
				})
				.then((posted) => {
					notify(sessionId, {
						sessionUpdate: "agent_message_chunk",
						content: {
							type: "text",
							// Echo the turn prompt (truncated) so steers and follow-up
							// deliveries are observable in the final result text.
							text: `ACP-EXTERNAL-AGENT: turn="${text.slice(0, 80)}"${slow ? " (slow mode)" : ""}; board post: ${posted}`,
						},
					})
					respond(msg.id, { stopReason: "end_turn" })
				})
				.catch((err) => {
					notify(sessionId, {
						sessionUpdate: "agent_message_chunk",
						content: { type: "text", text: `ACP-EXTERNAL-AGENT: failed (${err.message})` },
					})
					respond(msg.id, { stopReason: "end_turn" })
				})
			break
		}
		case "session/cancel": {
			// session/cancel arrives as a JSON-RPC notification (no id) — only
			// respond when it carries one, else the response orphans.
			if (msg.id !== undefined) respond(msg.id, {})
			break
		}
		default: {
			if (msg.id !== undefined) respond(msg.id, {})
		}
	}
}
