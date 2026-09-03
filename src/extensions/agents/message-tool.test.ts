import type { ExtensionAPI } from "@earendil-works/pi-coding-agent"
import { describe, expect, it, vi } from "vitest"
import {
	type AgentMessageCapability,
	createAgentMessageExtension,
	LIST_AGENT_CONTACTS_TOOL_NAME,
	POST_AGENT_NOTE_TOOL_NAME,
	READ_AGENT_BOARD_TOOL_NAME,
	SEND_AGENT_MESSAGE_TOOL_NAME,
} from "./message-tool.js"

function makePi() {
	const tools: Array<{ name: string; execute: (...args: unknown[]) => Promise<unknown> }> = []
	return {
		pi: { registerTool: vi.fn((tool) => tools.push(tool)) } as unknown as ExtensionAPI,
		tools,
	}
}

describe("agent communication child tools", () => {
	it("binds all four tools", () => {
		const capability: AgentMessageCapability = {
			listContacts: vi.fn(() => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			})),
			sendMessage: vi.fn(),
			postBoardEntry: vi.fn(),
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		expect(tools.map((tool) => tool.name)).toEqual([
			LIST_AGENT_CONTACTS_TOOL_NAME,
			SEND_AGENT_MESSAGE_TOOL_NAME,
			POST_AGENT_NOTE_TOOL_NAME,
			READ_AGENT_BOARD_TOOL_NAME,
		])
	})

	it("contacts list includes board hint when capability has board entries", async () => {
		const capability: AgentMessageCapability = {
			listContacts: vi.fn(() => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
				board: { total: 3, latestId: "bd-abc123" },
			})),
			sendMessage: vi.fn(),
			postBoardEntry: vi.fn(),
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[0]?.execute("tool-call")
		expect(result).toBeDefined()
		const text =
			typeof result === "object" && result !== null && "content" in result
				? (result as { content: Array<{ text: string }> }).content[0]?.text
				: ""
		expect(text).toContain('"board":{"total":3,"latestId":"bd-abc123"}')
		expect(capability.listContacts).toHaveBeenCalledOnce()
	})

	it("contacts list omits board when capability has no entries", async () => {
		const capability: AgentMessageCapability = {
			listContacts: vi.fn(() => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			})),
			sendMessage: vi.fn(),
			postBoardEntry: vi.fn(),
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[0]?.execute("tool-call")
		expect(result).toBeDefined()
		const text =
			typeof result === "object" && result !== null && "content" in result
				? (result as { content: Array<{ text: string }> }).content[0]?.text
				: ""
		expect(text).not.toContain('"board"')
	})

	it("validates input before invoking the host send callback", async () => {
		const sendMessage = vi.fn().mockResolvedValue({ status: "queued_for_parent" })
		const capability: AgentMessageCapability = {
			listContacts: () => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			}),
			sendMessage,
			postBoardEntry: vi.fn(),
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[1]?.execute("tool-call", {
			recipient: { type: "user" },
			payload: { kind: "status", summary: "not allowed for user" },
		})

		expect(result).toMatchObject({ content: [{ type: "text", text: expect.stringContaining("Message must use") }] })
		expect(sendMessage).not.toHaveBeenCalled()
	})

	it("returns the host receipt without claiming delivery", async () => {
		const sendMessage = vi.fn().mockResolvedValue({ status: "queued_for_parent", messageId: "m1", threadId: "m1" })
		const capability: AgentMessageCapability = {
			listContacts: () => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			}),
			sendMessage,
			postBoardEntry: vi.fn(),
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[1]?.execute("tool-call", {
			recipient: { type: "parent" },
			payload: { kind: "status", summary: "progress" },
		})

		expect(sendMessage).toHaveBeenCalledWith("tool-call", expect.objectContaining({ recipient: { type: "parent" } }))
		expect(result).toMatchObject({ content: [{ type: "text", text: expect.stringContaining("queued_for_parent") }] })
	})

	it("post_agent_note calls postBoardEntry with the capability", async () => {
		const postBoardEntry = vi.fn().mockReturnValue({
			ok: true as const,
			entry: { id: "bd-1234", kind: "note", title: "Test", body: "body", authorAgentId: "agent-1" },
			truncated: [],
		})
		const capability: AgentMessageCapability = {
			listContacts: () => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			}),
			sendMessage: vi.fn(),
			postBoardEntry,
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[2]?.execute("tool-call", {
			kind: "note",
			title: "Test note",
			body: "Body",
		})

		expect(postBoardEntry).toHaveBeenCalledWith({ kind: "note", title: "Test note", body: "Body" })
		expect(result).toMatchObject({ content: [{ type: "text", text: expect.stringContaining('"ok":true') }] })
	})

	it("post_agent_note rejects bad kind via schema", async () => {
		const postBoardEntry = vi.fn()
		const capability: AgentMessageCapability = {
			listContacts: () => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			}),
			sendMessage: vi.fn(),
			postBoardEntry,
			readBoardEntries: vi.fn(),
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[2]?.execute("tool-call", {
			kind: "invalid",
			title: "Test",
			body: "Body",
		})

		expect(postBoardEntry).not.toHaveBeenCalled()
		const text =
			typeof result === "object" && result !== null && "content" in result
				? (result as { content: Array<{ text: string }> }).content[0]?.text
				: ""
		expect(text).toContain('"ok":false')
	})

	it("read_agent_board passes since_id and kind filters", async () => {
		const readBoardEntries = vi.fn().mockReturnValue({
			ok: true as const,
			entries: [{ id: "bd-456", kind: "work", title: "Work", body: "done" }],
			total: 1,
		})
		const capability: AgentMessageCapability = {
			listContacts: () => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			}),
			sendMessage: vi.fn(),
			postBoardEntry: vi.fn(),
			readBoardEntries,
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[3]?.execute("tool-call", {
			since_id: "bd-123",
			kind: "work",
			limit: 10,
		})

		expect(readBoardEntries).toHaveBeenCalledWith({
			sinceId: "bd-123",
			kind: "work",
			limit: 10,
		})
		expect(result).toMatchObject({ content: [{ type: "text", text: expect.stringContaining('"ok":true') }] })
	})

	it("read_agent_board returns failure receipt for denied agent", async () => {
		const readBoardEntries = vi.fn().mockReturnValue({
			ok: false as const,
			reason: "not_authorized_for_board" as const,
		})
		const capability: AgentMessageCapability = {
			listContacts: () => ({
				parent: { reachable: true },
				user_via_parent: { reachable: false, route: "unavailable" as const },
				peers: [],
			}),
			sendMessage: vi.fn(),
			postBoardEntry: vi.fn(),
			readBoardEntries,
		}
		const { pi, tools } = makePi()
		createAgentMessageExtension(capability)(pi)

		const result = await tools[3]?.execute("tool-call", {})

		expect(readBoardEntries).toHaveBeenCalledWith({})
		expect(result).toMatchObject({ content: [{ type: "text", text: expect.stringContaining('"ok":false') }] })
	})
})
