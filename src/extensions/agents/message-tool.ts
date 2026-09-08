import { defineTool, type ExtensionAPI } from "@earendil-works/pi-coding-agent"
import { Type } from "typebox"
import { Value } from "typebox/value"
import {
	BOARD_ENTRY_BODY_MAX,
	BOARD_ENTRY_TITLE_MAX,
	type BoardEntryKind,
	type BoardPostReceipt,
	type BoardReadReceipt,
} from "./manager/board.js"
import {
	type AgentMessageInput,
	AgentMessageInputSchema,
	type AgentMessageReceipt,
	validateAgentMessageInput,
} from "./messages.js"
import { textResult } from "./tool-result.js"

export const LIST_AGENT_CONTACTS_TOOL_NAME = "list_agent_contacts"
export const SEND_AGENT_MESSAGE_TOOL_NAME = "send_agent_message"
export const POST_AGENT_NOTE_TOOL_NAME = "post_agent_note"
export const READ_AGENT_BOARD_TOOL_NAME = "read_agent_board"

export interface AgentContact {
	agent_id?: string
	task_id?: string
	persona?: string
	description?: string
	status?: string
	reachable: boolean
	route?: "parent" | "peer" | "questionnaire" | "ferment_judge" | "unavailable"
	ferment_id?: string
	reason?: string
}

export interface AgentContactList {
	parent: AgentContact
	user_via_parent: AgentContact
	peers: AgentContact[]
	/** Board hint for the caller's group — present when the caller has an active group with board entries. */
	board?: { total: number; latestId?: string }
}

export interface AgentMessageCapability {
	listContacts(): AgentContactList
	sendMessage(toolCallId: string, input: AgentMessageInput): Promise<AgentMessageReceipt>
	/** Post a note/work/finding/warning to the shared coordination board. */
	postBoardEntry(input: { kind: BoardEntryKind; title: string; body: string }): BoardPostReceipt
	/** Read board entries for the caller's group. */
	readBoardEntries(opts?: { sinceId?: string; kind?: BoardEntryKind; limit?: number }): BoardReadReceipt
}

export const PostAgentNoteSchema = Type.Object(
	{
		kind: Type.Enum({
			note: "note" as const,
			work: "work" as const,
			finding: "finding" as const,
			warning: "warning" as const,
		}),
		title: Type.String({ maxLength: BOARD_ENTRY_TITLE_MAX }),
		body: Type.String({ maxLength: BOARD_ENTRY_BODY_MAX }),
	},
	{ additionalProperties: false },
)

export const ReadAgentBoardSchema = Type.Object(
	{
		since_id: Type.Optional(Type.String()),
		kind: Type.Optional(
			Type.Enum({
				note: "note" as const,
				work: "work" as const,
				finding: "finding" as const,
				warning: "warning" as const,
			}),
		),
		limit: Type.Optional(Type.Number({ minimum: 1, maximum: 200 })),
	},
	{ additionalProperties: false },
)

export function createAgentMessageExtension(capability: AgentMessageCapability): (pi: ExtensionAPI) => void {
	return (pi) => {
		pi.registerTool(
			defineTool({
				name: LIST_AGENT_CONTACTS_TOOL_NAME,
				label: "List Agent Contacts",
				description:
					"List recipients the host currently authorizes for this agent. Call again if peer state may have changed.",
				parameters: Type.Object({}, { additionalProperties: false }),
				execute: async () => textResult(JSON.stringify(capability.listContacts())),
			}),
		)

		pi.registerTool(
			defineTool({
				name: SEND_AGENT_MESSAGE_TOOL_NAME,
				label: "Send Agent Message",
				description:
					"Send one focused message to an authorized contact. A receipt proves only host queue acceptance or a completed bounded resume attempt; it does not prove delivery or recipient action.",
				parameters: AgentMessageInputSchema,
				execute: async (toolCallId, params) => {
					const validated = validateAgentMessageInput(params)
					if (!validated.valid) return textResult(validated.reason)
					const receipt = await capability.sendMessage(toolCallId, validated.value)
					return textResult(JSON.stringify(receipt))
				},
			}),
		)

		pi.registerTool(
			defineTool({
				name: POST_AGENT_NOTE_TOOL_NAME,
				label: "Post Agent Note",
				description:
					"Post a note/work/finding/warning to the shared coordination board for the agent group. " +
					"The board is shared append-only group space; use send_agent_message for directed 1:1 communication.",
				parameters: PostAgentNoteSchema,
				execute: async (_toolCallId, params) => {
					if (!Value.Check(PostAgentNoteSchema, params)) {
						return textResult(JSON.stringify({ ok: false, reason: "invalid_schema" }))
					}
					return textResult(JSON.stringify(capability.postBoardEntry(params)))
				},
			}),
		)

		pi.registerTool(
			defineTool({
				name: READ_AGENT_BOARD_TOOL_NAME,
				label: "Read Agent Board",
				description:
					"Read new board entries since an id, filtered by kind, up to a limit. " +
					"Returns only authorized entries for the caller's group. " +
					"Pass since_id on re-reads to get only new entries.",
				parameters: ReadAgentBoardSchema,
				execute: async (_toolCallId, params) => {
					if (!Value.Check(ReadAgentBoardSchema, params)) {
						return textResult(JSON.stringify({ ok: false, reason: "invalid_schema" }))
					}
					const opts: Parameters<typeof capability.readBoardEntries>[0] = {
						sinceId: (params as { since_id?: string }).since_id,
						kind: (params as { kind?: BoardEntryKind }).kind,
						limit: (params as { limit?: number }).limit,
					}
					return textResult(JSON.stringify(capability.readBoardEntries(opts)))
				},
			}),
		)
	}
}
