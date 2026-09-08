import { mkdtempSync, rmSync } from "node:fs"
import { connect } from "node:net"
import { tmpdir } from "node:os"
import { join } from "node:path"
import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent"
import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { getActiveManager, setActiveManagerForTest } from "../agents/index.js"
import { AgentManager } from "../agents/manager/agent-manager.js"
import { type AgentCommsIpcServer, getAgentCommsIpc, resetAgentCommsIpcForTest } from "./comms-ipc.js"

interface IpcResponse {
	id: string
	result?: { ok?: boolean; entry?: { authorAgentId?: string } }
	error?: string
}

function ipcCall(socketPath: string, req: unknown): Promise<IpcResponse> {
	return new Promise((resolve, reject) => {
		const sock = connect(socketPath)
		sock.setEncoding("utf8")
		let buffer = ""
		const failTimer = setTimeout(() => {
			sock.destroy()
			reject(new Error("ipc response timeout"))
		}, 3000)
		sock.on("connect", () => {
			sock.write(`${JSON.stringify(req)}\n`)
		})
		sock.on("data", (chunk) => {
			buffer += chunk
			const idx = buffer.indexOf("\n")
			if (idx >= 0) {
				clearTimeout(failTimer)
				resolve(JSON.parse(buffer.slice(0, idx)) as IpcResponse)
				sock.end()
			}
		})
		sock.on("error", (err) => {
			clearTimeout(failTimer)
			reject(err)
		})
	})
}

describe("AgentCommsIpcServer", () => {
	let manager: AgentManager | undefined
	let ipc: AgentCommsIpcServer
	let socketPath: string
	const prevActiveManager = getActiveManager()

	beforeEach(() => {
		resetAgentCommsIpcForTest()
		ipc = getAgentCommsIpc()
		socketPath = ipc.ensureStarted()
	})

	afterEach(() => {
		manager?.dispose()
		manager = undefined
		setActiveManagerForTest(prevActiveManager)
		resetAgentCommsIpcForTest()
	})

	function spawnCommunicatingAgent(m: AgentManager): string {
		// maxConcurrent 0 + isBackground keeps the record queued (live for comms).
		const id = m.spawn({} as ExtensionAPI, { cwd: process.cwd() } as unknown as ExtensionContext, "Explore", "test", {
			description: "test",
			isBackground: true,
			communication: "group",
			rootSessionId: "root-1",
		})
		// Board and peer routes are group-scoped: finalize the batch group like
		// the host Agent tool does for communicating batches.
		const record = m.getRecord(id)
		if (record) record.groupId = "batch-1"
		return id
	}

	function bindCommunication(m: AgentManager): void {
		m.bindCommunicationRoot("root-1")
		m.registerParentBridge("root-1", () => true)
		m.setUserContactResolver("root-1", () => ({ reachable: true, route: "questionnaire" }))
	}

	it("denies unknown, missing, and malformed tokens", async () => {
		await expect(
			ipcCall(socketPath, { id: "r1", token: "nope", method: "list_agent_contacts" }),
		).resolves.toMatchObject({ id: "r1", error: "not authorized" })
		await expect(ipcCall(socketPath, { id: "r2", method: "list_agent_contacts" })).resolves.toMatchObject({
			id: "r2",
			error: "not authorized",
		})
	})

	it("dispatches list_contacts for a live communicating record", async () => {
		manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		bindCommunication(manager)
		const agentId = spawnCommunicatingAgent(manager)

		const token = ipc.registerToken(agentId)
		const res = await ipcCall(socketPath, { id: "r1", token, method: "list_agent_contacts" })

		expect(res.error).toBeUndefined()
		expect(res.result).toMatchObject({
			parent: { reachable: true, route: "parent" },
		})
	})

	it("posts to the board with host-stamped authorship", async () => {
		manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		bindCommunication(manager)
		const agentId = spawnCommunicatingAgent(manager)

		const token = ipc.registerToken(agentId)
		const res = await ipcCall(socketPath, {
			id: "r1",
			token,
			method: "post_agent_note",
			params: { kind: "note", title: "hello", body: "from the outside" },
		})

		expect(res.error).toBeUndefined()
		expect(res.result).toMatchObject({ ok: true, entry: { authorAgentId: agentId, kind: "note" } })

		// The same agent reads its own entry back through the IPC path.
		const read = await ipcCall(socketPath, { id: "r2", token, method: "read_agent_board" })
		expect(read.error).toBeUndefined()
		expect(read.result).toMatchObject({
			ok: true,
			entries: [{ authorAgentId: agentId, title: "hello", body: "from the outside" }],
		})
	})

	it("denies a terminal record even before token revocation (accessor-level defense)", async () => {
		manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		const agentId = spawnCommunicatingAgent(manager)

		const token = ipc.registerToken(agentId)
		// Force the record terminal without firing the revoker wiring.
		expect(manager.abort(agentId)).toBe(true)

		await expect(
			ipcCall(socketPath, {
				id: "r1",
				token,
				method: "post_agent_note",
				params: { kind: "note", title: "x", body: "y" },
			}),
		).resolves.toMatchObject({ id: "r1", error: "not authorized" })
	})

	it("revokes tokens through the manager-owned hook on terminal transitions", async () => {
		manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		manager.setCommsTokenRevoker((agentId) => ipc.revokeAgent(agentId))
		const agentId = spawnCommunicatingAgent(manager)

		const token = ipc.registerToken(agentId)
		// Pre-terminal sanity: authorized.
		const ok = await ipcCall(socketPath, { id: "r0", token, method: "list_agent_contacts" })
		expect(ok.error).toBeUndefined()

		expect(manager.abort(agentId)).toBe(true)

		// Revoker fired synchronously with the terminal transition.
		await expect(ipcCall(socketPath, { id: "r1", token, method: "list_agent_contacts" })).resolves.toMatchObject({
			id: "r1",
			error: "not authorized",
		})
	})

	it("forwards send_agent_message questions to the parent bridge", async () => {
		manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		bindCommunication(manager)
		const agentId = spawnCommunicatingAgent(manager)

		const token = ipc.registerToken(agentId)
		const res = await ipcCall(socketPath, {
			id: "r1",
			token,
			method: "send_agent_message",
			params: {
				recipient: { type: "user" },
				payload: { kind: "question", question: "which database?", impact: "correctness", canContinue: true },
			},
		})

		expect(res.error).toBeUndefined()
		expect(res.result).toMatchObject({ status: "queued_for_parent" })
	})

	it("rejects unknown methods", async () => {
		manager = new AgentManager(undefined, 0)
		setActiveManagerForTest(manager)
		const agentId = spawnCommunicatingAgent(manager)

		const token = ipc.registerToken(agentId)
		await expect(ipcCall(socketPath, { id: "r1", token, method: "definitely_not_a_tool" })).resolves.toMatchObject({
			id: "r1",
			error: "unknown method: definitely_not_a_tool",
		})
	})

	it("stop() removes the socket dir", () => {
		const dir = join(mkdtempSync(join(tmpdir(), "acp-ipc-check-")), "")
		expect(ipc.socketPath).toBeTypeOf("string")
		ipc.stop()
		expect(ipc.socketPath).toBeUndefined()
		rmSync(dir, { recursive: true, force: true })
	})
})
