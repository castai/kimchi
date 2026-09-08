import { type ChildProcess, spawn } from "node:child_process"
import { mkdirSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import { expect, test } from "@microsoft/tui-test"
import { STREAM_TIMEOUT_MS, viewText, waitForText } from "./support/assertions.js"
import { runKimchiSession, TUI_TEST_CONFIG } from "./support/kimchi-fixture.js"

test.use(TUI_TEST_CONFIG)

const fakeAcpAgentPath = new URL("./support/fake-acp-agent.mjs", import.meta.url).pathname
const wsAgentPath = new URL("./support/fake-acp-agent-ws.mjs", import.meta.url).pathname

/** Start the WS ACP fixture on a random port; resolves with the port. */
function startWsFixture(): Promise<{ child: ChildProcess; port: number }> {
	const child = spawn(process.execPath, [wsAgentPath, "0"], { stdio: ["ignore", "pipe", "inherit"] })
	return new Promise((resolve, reject) => {
		const timer = setTimeout(() => reject(new Error("ws fixture did not report ready")), 15_000)
		child.stdout?.setEncoding("utf8")
		child.stdout?.on("data", (chunk: string) => {
			const match = chunk.match(/WS-ACP-READY (\d+)/)
			if (match) {
				clearTimeout(timer)
				resolve({ child, port: Number(match[1]) })
			}
		})
		child.on("exit", (code) => {
			clearTimeout(timer)
			reject(new Error(`ws fixture exited early: ${code}`))
		})
	})
}

test("ACP external agent joins the subagent plane and posts to the coordination board", async ({ terminal }) => {
	await runKimchiSession(
		terminal,
		{
			artifactName: "acp-subagent-board",
			// The acp-agents extension (types, runner, comms shim) only exists
			// behind the experimental flag.
			extraArgs: ["--enable-experimental-features"],
			seedHome: (_homeDir, workDir) => {
				const agentsDir = join(workDir, ".kimchi", "agents")
				mkdirSync(agentsDir, { recursive: true })
				writeFileSync(
					join(agentsDir, "settle-worker.md"),
					"---\ndescription: settle worker\nprompt_mode: append\nextensions: true\nskills: false\n---\nFollow the task instructions exactly.",
					"utf-8",
				)
				writeFileSync(
					join(workDir, ".kimchi", "acp-agents.json"),
					JSON.stringify({
						agent_servers: {
							fake: { command: process.execPath, args: [fakeAcpAgentPath] },
						},
					}),
					"utf-8",
				)
			},
			models: [{ slug: "basic", displayName: "Fake Basic", input: ["text"] }],
			responses: [
				{
					// One communicating batch: the board is group-scoped, so the
					// external ACP agent joins it together with an in-process
					// settle worker — the same batch contract as the board e2e.
					toolCalls: [
						{
							id: "call_spawn_inprocess",
							function: {
								name: "Agent",
								arguments: JSON.stringify({
									prompt: "IN-PROCESS task: settle immediately.",
									description: "in-process settle worker",
									subagent_type: "settle-worker",
									communication: "group",
									run_in_background: true,
								}),
							},
						},
						{
							id: "call_spawn_acp",
							function: {
								name: "Agent",
								arguments: JSON.stringify({
									prompt: "Post one finding to the coordination board, then finish.",
									description: "external ACP worker",
									subagent_type: "acp:fake",
									communication: "group",
									run_in_background: true,
								}),
							},
						},
					],
				},
				{ stream: ["coordinator: batch spawned"] },
				{
					// Fires only when the external agent's board entry reaches the
					// coordinator through the coordination-board-update follow-up:
					// the streamed/result text deliberately omits the entry title.
					match: (request) => {
						const body = JSON.stringify(request.body ?? {})
						return body.includes("ACP-EXTERNAL-POSTED")
					},
					stream: ["ACP-WORKFLOW-DONE: external board entry observed by the coordinator"],
				},
				{
					// The in-process settle worker's turn.
					forSubagent: true,
					match: (request) => {
						const body = JSON.stringify(request.body ?? {})
						return body.includes("IN-PROCESS task")
					},
					stream: ["IN-PROCESS-DONE: settled"],
				},
			],
		},
		async (_fixture, trace) => {
			terminal.submit("connect the fake ACP agent through ACP")
			// The external agent streams like any subagent in the TUI tree.
			await waitForText(terminal, "ACP-EXTERNAL-AGENT", { timeoutMs: 120_000 })
			trace.step("external ACP agent streams in the TUI subagent tree")
			await waitForText(terminal, "ACP-WORKFLOW-DONE", { timeoutMs: STREAM_TIMEOUT_MS })
			trace.step("coordinator observes the external agent's board entry via the follow-up digest")
			const view = viewText(terminal)
			expect(view).toContain("ACP-EXTERNAL-AGENT")
			expect(view).toContain("ACP-WORKFLOW-DONE")
		},
	)
})

test("two external ACP agents exchange a question and answer through the host", async ({ terminal }) => {
	await runKimchiSession(
		terminal,
		{
			artifactName: "acp-subagent-external-to-external",
			extraArgs: ["--enable-experimental-features"],
			seedHome: (_homeDir, workDir) => {
				mkdirSync(join(workDir, ".kimchi"), { recursive: true })
				writeFileSync(
					join(workDir, ".kimchi", "acp-agents.json"),
					JSON.stringify({
						agent_servers: {
							fake: { command: process.execPath, args: [fakeAcpAgentPath] },
						},
					}),
					"utf-8",
				)
			},
			models: [{ slug: "basic", displayName: "Fake Basic", input: ["text"] }],
			responses: [
				{
					// One communicating batch of two EXTERNAL agents: the asker
					// (ASK_PEER fixture mode) discovers and questions the responder
					// (WAIT fixture mode) purely through the host MCP shim.
					toolCalls: [
						{
							id: "call_spawn_asker",
							function: {
								name: "Agent",
								arguments: JSON.stringify({
									prompt: "ASK_PEER: find a peer and ask it one question, then wait for its answer.",
									description: "external asker",
									subagent_type: "acp:fake",
									communication: "group",
									run_in_background: true,
								}),
							},
						},
						{
							id: "call_spawn_responder",
							function: {
								name: "Agent",
								arguments: JSON.stringify({
									prompt: "WAIT a while, post one finding to the coordination board, then finish.",
									description: "external responder",
									subagent_type: "acp:fake",
									communication: "group",
									run_in_background: true,
								}),
							},
						},
					],
				},
				{ stream: ["coordinator: both external agents spawned"] },
				{
					// Fires when the responder's final result (it answered the asker
					// through the MCP shim) reaches the coordinator.
					match: (request) => {
						const body = JSON.stringify(request.body ?? {})
						return body.includes("ACP-PEER-ANSWERED")
					},
					stream: ["EXTERNAL-TO-EXTERNAL-DONE: the asker received the responder's answer"],
				},
			],
		},
		async (_fixture, trace) => {
			terminal.submit("connect two external ACP agents that talk to each other")
			// The responder's completion notification carries its answer receipt.
			await waitForText(terminal, "ACP-PEER-ANSWERED", { timeoutMs: 120_000 })
			trace.step("external responder answered the external asker through the host")
			await waitForText(terminal, "EXTERNAL-TO-EXTERNAL-DONE", { timeoutMs: STREAM_TIMEOUT_MS })
			const view = viewText(terminal)
			expect(view).toContain("ACP-PEER-ANSWERED")
			expect(view).toContain("EXTERNAL-TO-EXTERNAL-DONE")
		},
	)
})

test("steer_subagent redirects a running external ACP agent", async ({ terminal }) => {
	await runKimchiSession(
		terminal,
		{
			artifactName: "acp-subagent-steer",
			extraArgs: ["--enable-experimental-features"],
			seedHome: (_homeDir, workDir) => {
				mkdirSync(join(workDir, ".kimchi"), { recursive: true })
				writeFileSync(
					join(workDir, ".kimchi", "acp-agents.json"),
					JSON.stringify({
						agent_servers: {
							fake: { command: process.execPath, args: [fakeAcpAgentPath] },
						},
					}),
					"utf-8",
				)
			},
			models: [{ slug: "basic", displayName: "Fake Basic", input: ["text"] }],
			responses: [
				{
					toolCalls: [
						{
							id: "call_spawn",
							function: {
								name: "Agent",
								arguments: JSON.stringify({
									prompt: "WAIT a while, post one finding to the coordination board, then finish.",
									description: "external ACP steer target",
									subagent_type: "acp:fake",
									communication: "group",
									run_in_background: true,
								}),
							},
						},
					],
				},
				{
					// The request after the spawn carries the agent id in the tool
					// result — steer it while it is still mid WAIT turn.
					match: (request) => {
						const body = JSON.stringify(request.body ?? {})
						return body.includes("external ACP steer target")
					},
					toolCalls: [
						{
							id: "call_steer",
							function: {
								name: "steer_subagent",
								arguments: JSON.stringify({
									agent_id: "__AGENT_ID__",
									message: "STEER-CHECK: acknowledge this steering message in your final output.",
								}),
							},
						},
					],
				},
				{
					// Fires when the agent's final result — the turn echo of the
					// drained steer — reaches the coordinator.
					match: (request) => {
						const body = JSON.stringify(request.body ?? {})
						// The steer tool result mentions the message but never
						// "ACP-EXTERNAL-AGENT"; only the agent's final result does.
						return body.includes("ACP-EXTERNAL-AGENT") && body.includes("STEER-CHECK")
					},
					stream: ["STEER-DELIVERY-DONE: the steer reached the external agent's final output"],
				},
			],
		},
		async (_fixture, trace) => {
			terminal.submit("steer an external ACP agent mid-run")
			await waitForText(terminal, "STEER-CHECK", { timeoutMs: 120_000 })
			trace.step("the steer was drained as a follow-up ACP turn and echoed in the result")
			await waitForText(terminal, "STEER-DELIVERY-DONE", { timeoutMs: STREAM_TIMEOUT_MS })
			const view = viewText(terminal)
			expect(view).toContain("STEER-CHECK")
			expect(view).toContain("STEER-DELIVERY-DONE")
		},
	)
})

test("WebSocket-transport ACP agent posts to the coordination board through the comms shim", async ({ terminal }) => {
	// The WS fixture must outlive the kimchi session — start it here, kill it in
	// the finally below.
	const { child: wsChild, port } = await startWsFixture()
	try {
		await runKimchiSession(
			terminal,
			{
				artifactName: "acp-subagent-ws-transport",
				extraArgs: ["--enable-experimental-features"],
				seedHome: (_homeDir, workDir) => {
					const agentsDir = join(workDir, ".kimchi", "agents")
					mkdirSync(agentsDir, { recursive: true })
					writeFileSync(
						join(agentsDir, "settle-worker.md"),
						"---\ndescription: settle worker\nprompt_mode: append\nextensions: true\nskills: false\n---\nFollow the task instructions exactly, then finish quickly.",
						"utf-8",
					)
					mkdirSync(join(workDir, ".kimchi"), { recursive: true })
					writeFileSync(
						join(workDir, ".kimchi", "acp-agents.json"),
						JSON.stringify({
							agent_servers: {
								fakews: { transport: "ws", url: `ws://127.0.0.1:${port}` },
							},
						}),
						"utf-8",
					)
				},
				models: [{ slug: "basic", displayName: "Fake Basic", input: ["text"] }],
				responses: [
					{
						// One communicating batch: the WS agent + an in-process partner
						// (board access requires a batch of ≥2).
						toolCalls: [
							{
								id: "call_spawn_ws",
								function: {
									name: "Agent",
									arguments: JSON.stringify({
										prompt: "Post one finding to the coordination board, then finish.",
										description: "external WS ACP worker",
										subagent_type: "acp:fakews",
										communication: "group",
										run_in_background: true,
									}),
								},
							},
							{
								id: "call_spawn_partner",
								function: {
									name: "Agent",
									arguments: JSON.stringify({
										prompt: "Settle immediately.",
										description: "ws batch partner",
										subagent_type: "settle-worker",
										communication: "group",
										run_in_background: true,
									}),
								},
							},
						],
					},
					{ stream: ["coordinator: WS batch spawned"] },
					{
						match: (request) => {
							const body = JSON.stringify(request.body ?? {})
							return body.includes("ACP-WS-POSTED")
						},
						stream: ["WS-TRANSPORT-DONE: the WS agent's board entry was observed"],
					},
				],
			},
			async (_fixture, trace) => {
				terminal.submit("connect the WS-transport ACP agent")
				await waitForText(terminal, "ACP-WS-AGENT", { timeoutMs: 120_000 })
				trace.step("WS-transport external agent connected and streamed in the TUI")
				await waitForText(terminal, "WS-TRANSPORT-DONE", { timeoutMs: STREAM_TIMEOUT_MS })
				const view = viewText(terminal)
				expect(view).toContain("ACP-WS-AGENT")
				expect(view).toContain("WS-TRANSPORT-DONE")
			},
		)
	} finally {
		wsChild.kill("SIGTERM")
	}
})
