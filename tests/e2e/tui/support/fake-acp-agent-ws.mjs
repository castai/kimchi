/**
 * fake-acp-agent-ws.mjs — WebSocket ACP agent for live transport testing.
 *
 * Listens on ws://127.0.0.1:<port> and accepts any path (the host client
 * connects to `${url}/session/${sessionName}/connect`). Each WebSocket
 * connection is one ACP JSON-RPC session: initialize, session/new (spawns the
 * MCP comms shim from the passed mcpServers, like the stdio fixture), and
 * session/prompt (post a finding to the coordination board through the shim,
 * stream one line, end turn).
 *
 * Prints "WS-ACP-READY <port>" once listening so the driver can wait for it.
 */

import { spawn } from "node:child_process"
import { WebSocketServer } from "ws"

const port = Number(process.argv[2] ?? 0)

let shim = null
let mcpId = 0
const pendingMcp = new Map()
let shimReady = null

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

async function mcpCall(name, arguments_) {
	const res = await mcpRequest("tools/call", { name, arguments: arguments_ })
	return res?.result?.content?.[0]?.text ?? ""
}

function startShim(mcpServers) {
	const server = mcpServers?.[0]
	// No mcpServers means the host spawned this agent without communication:
	// skip the board post and answer plainly (still validates the transport).
	if (!server) return Promise.resolve()
	shim = spawn(server.command, server.args ?? [], { stdio: ["pipe", "pipe", "ignore"] })
	shim.stdout.setEncoding("utf8")
	const lines = []
	shim.stdout.on("data", (chunk) => {
		lines.push(...chunk.split("\n"))
		const rest = lines.pop() ?? ""
		for (const line of lines.splice(0)) {
			const trimmed = line.trim()
			if (!trimmed) continue
			let res
			try {
				res = JSON.parse(trimmed)
			} catch {
				continue
			}
			if (res.id !== undefined && pendingMcp.has(res.id)) {
				pendingMcp.get(res.id).resolve(res)
				pendingMcp.delete(res.id)
			}
		}
		lines.push(rest)
	})
	return mcpRequest("initialize", {
		protocolVersion: "2024-11-05",
		capabilities: {},
		clientInfo: { name: "fake-acp-agent-ws", version: "1.0.0" },
	}).then(() => {
		shim.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" })}\n`)
	})
}

async function postFindingToBoard() {
	if (!shim) return "skipped (no comms shim - communication disabled)"
	const text = await mcpCall("post_agent_note", {
		kind: "finding",
		title: "ACP-WS-POSTED",
		body: "Posted by the external WS-transport ACP agent through the host comms shim.",
	})
	return text.includes('"ok":true') ? "yes" : `no (${text.slice(0, 120)})`
}

function handle(ws, msg) {
	const send = (obj) => ws.send(`${JSON.stringify(obj)}\n`)
	switch (msg.method) {
		case "initialize": {
			send({
				jsonrpc: "2.0",
				id: msg.id,
				result: { protocolVersion: msg.params?.protocolVersion ?? "1", agentCapabilities: {} },
			})
			break
		}
		case "session/new": {
			shimReady = startShim(msg.params?.mcpServers).catch((err) => {
				process.stderr.write(`shim failed: ${err.message}\n`)
			})
			send({ jsonrpc: "2.0", id: msg.id, result: { sessionId: "acp-ws-1" } })
			break
		}
		case "session/prompt": {
			const sessionId = msg.params.sessionId
			Promise.resolve(shimReady)
				.then(() => postFindingToBoard())
				.then((posted) => {
					send({
						jsonrpc: "2.0",
						method: "session/update",
						params: {
							sessionId,
							update: {
								sessionUpdate: "agent_message_chunk",
								content: { type: "text", text: `ACP-WS-AGENT: connected over WebSocket; board post: ${posted}` },
							},
						},
					})
					send({ jsonrpc: "2.0", id: msg.id, result: { stopReason: "end_turn" } })
				})
				.catch((err) => {
					send({
						jsonrpc: "2.0",
						method: "session/update",
						params: {
							sessionId,
							update: {
								sessionUpdate: "agent_message_chunk",
								content: { type: "text", text: `ACP-WS-AGENT: failed (${err.message})` },
							},
						},
					})
					send({ jsonrpc: "2.0", id: msg.id, result: { stopReason: "end_turn" } })
				})
			break
		}
		case "session/cancel": {
			if (msg.id !== undefined) send({ jsonrpc: "2.0", id: msg.id, result: {} })
			break
		}
		default: {
			if (msg.id !== undefined) send({ jsonrpc: "2.0", id: msg.id, result: {} })
		}
	}
}

const wss = new WebSocketServer({ host: "127.0.0.1", port })
wss.on("connection", (ws) => {
	ws.on("message", (data) => {
		const text = data.toString().trim()
		if (!text) return
		try {
			handle(ws, JSON.parse(text))
		} catch {
			/* malformed line — ignore */
		}
	})
	ws.on("error", () => ws.terminate())
})
wss.on("listening", () => {
	const actualPort = wss.address().port
	process.stdout.write(`WS-ACP-READY ${actualPort}\n`)
})
