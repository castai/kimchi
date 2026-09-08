/**
 * comms-ipc.ts — host-side IPC server for ACP agent communication tools.
 *
 * One singleton Unix-domain socket per host process; per-agent random tokens
 * (registered by the ACP runner when the record has communication enabled).
 * The external agent's MCP shim (`--agent-comms-mcp`) connects and forwards
 * tool calls; every request re-validates: token → agentId → live
 * (running/queued) communicating record, then dispatches to the same
 * capability the in-process tools use. Author identity is host-stamped from
 * the record — the external model can never assert it.
 *
 * Token revocation is manager-owned: `AgentManager.transitionToTerminalRecord`
 * fires the registered revoker synchronously with the status transition, so a
 * terminal external agent cannot post during the race window.
 *
 * Line-delimited JSON over the socket:
 *   request:  { "id": <shim id>, "token": <token>, "method": <tool name>, "params": {...} }
 *   response: { "id": <same>, "result": <capability result> } | { "id": ..., "error": "..." }
 */

import { randomBytes } from "node:crypto"
import { mkdtempSync, rmSync } from "node:fs"
import { createServer, type Server, type Socket } from "node:net"
import { tmpdir } from "node:os"
import { join } from "node:path"
import type { BoardEntryKind } from "../agents/manager/board.js"
import type { AgentMessageInput } from "../agents/messages.js"

interface IpcRequest {
	id: string
	token: string
	method: string
	params?: unknown
}

export interface IpcResponse {
	id: string
	result?: unknown
	error?: string
}

/** Line-delimited JSON framing shared by the IPC server and the MCP shim:
 *  feed string chunks, parsed messages come out; malformed lines are ignored. */
export function createJsonLineReader(onMessage: (message: unknown) => void): (chunk: string) => void {
	let buffer = ""
	return (chunk) => {
		const lines = (buffer + chunk).split("\n")
		buffer = lines.pop() ?? ""
		for (const raw of lines) {
			const line = raw.trim()
			if (!line) continue
			try {
				onMessage(JSON.parse(line))
			} catch {
				// Malformed line — ignore; the peer's request eventually times out.
			}
		}
	}
}

export type AgentCommsIpcOutcome = { ok: true; result: unknown } | { ok: false; error: string }

export class AgentCommsIpcServer {
	private server?: Server
	private socketDir?: string
	private _socketPath?: string
	private tokens = new Map<string, string>()

	/** Socket path, set after ensureStarted(). */
	get socketPath(): string | undefined {
		return this._socketPath
	}

	/** Start the singleton socket (idempotent). Returns the socket path. */
	ensureStarted(): string {
		if (this._socketPath) return this._socketPath
		this.socketDir = mkdtempSync(join(tmpdir(), "kimchi-acp-comms-"))
		this._socketPath = join(this.socketDir, "comms.sock")
		this.server = createServer((socket) => this.handleConnection(socket))
		this.server.listen(this._socketPath)
		return this._socketPath
	}

	/** Register a fresh token for an agent; returns the token. */
	registerToken(agentId: string): string {
		const token = randomBytes(24).toString("hex")
		this.tokens.set(token, agentId)
		return token
	}

	/** Remove every token bound to an agent (manager-owned terminal revocation). */
	revokeAgent(agentId: string): void {
		for (const [token, id] of this.tokens) {
			if (id === agentId) this.tokens.delete(token)
		}
	}

	private handleConnection(socket: Socket): void {
		socket.setEncoding("utf8")
		socket.on(
			"data",
			createJsonLineReader((message) => {
				const req = message as IpcRequest
				this.dispatch(req)
					.then((outcome) => {
						const res: IpcResponse = outcome.ok
							? { id: req.id, result: outcome.result }
							: { id: req.id, error: outcome.error }
						socket.write(`${JSON.stringify(res)}\n`)
					})
					.catch((err: unknown) => {
						const res: IpcResponse = {
							id: req.id,
							error: err instanceof Error ? err.message : String(err),
						}
						socket.write(`${JSON.stringify(res)}\n`)
					})
			}),
		)
		socket.on("error", () => socket.destroy())
	}

	private async dispatch(req: IpcRequest): Promise<AgentCommsIpcOutcome> {
		if (typeof req.token !== "string" || typeof req.method !== "string" || typeof req.id !== "string") {
			return { ok: false, error: "not authorized" }
		}
		const agentId = this.tokens.get(req.token)
		if (!agentId) return { ok: false, error: "not authorized" }
		// Lazy import keeps this module loadable without the heavy agents index.
		const { getActiveManager } = await import("../agents/index.js")
		const capability = getActiveManager()?.getAgentCommsCapability(agentId)
		if (!capability) return { ok: false, error: "not authorized" }
		try {
			switch (req.method) {
				case "list_agent_contacts":
					return { ok: true, result: capability.listContacts() }
				case "send_agent_message": {
					// The shim's request id becomes the idempotency toolCallId.
					const receipt = await capability.sendMessage(`mcp:${req.id}`, req.params as AgentMessageInput)
					return { ok: true, result: receipt }
				}
				case "post_agent_note":
					return {
						ok: true,
						result: capability.postBoardEntry(req.params as { kind: BoardEntryKind; title: string; body: string }),
					}
				case "read_agent_board":
					return {
						ok: true,
						result: capability.readBoardEntries(
							req.params as { sinceId?: string; kind?: BoardEntryKind; limit?: number },
						),
					}
				default:
					return { ok: false, error: `unknown method: ${req.method}` }
			}
		} catch (err) {
			return { ok: false, error: err instanceof Error ? err.message : String(err) }
		}
	}

	stop(): void {
		this.server?.close()
		this.server = undefined
		if (this.socketDir) rmSync(this.socketDir, { recursive: true, force: true })
		this.socketDir = undefined
		this._socketPath = undefined
		this.tokens.clear()
	}
}

/** Module singleton — one IPC socket per host process. */
let ipcSingleton: AgentCommsIpcServer | undefined

export function getAgentCommsIpc(): AgentCommsIpcServer {
	if (!ipcSingleton) ipcSingleton = new AgentCommsIpcServer()
	return ipcSingleton
}

/** Test helper: stop and drop the singleton. */
export function resetAgentCommsIpcForTest(): void {
	ipcSingleton?.stop()
	ipcSingleton = undefined
}
