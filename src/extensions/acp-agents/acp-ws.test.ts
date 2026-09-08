import { type ChildProcess, spawn } from "node:child_process"
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import type { ExtensionContext } from "@earendil-works/pi-coding-agent"
import { afterEach, beforeEach, describe, expect, it } from "vitest"
import type { AgentRecord } from "../agents/personas/types.js"
import { runAcpAgent } from "./acp-runner.js"
import { resetAgentCommsIpcForTest } from "./comms-ipc.js"

const wsFixturePath = new URL("../../../tests/e2e/tui/support/fake-acp-agent-ws.mjs", import.meta.url).pathname

/** Start the WebSocket ACP fixture on a random port; resolves with the port. */
function startWsFixture(): Promise<{ child: ChildProcess; port: number }> {
	const child = spawn(process.execPath, [wsFixturePath, "0"], { stdio: ["ignore", "pipe", "inherit"] })
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

describe("ACP WebSocket transport", () => {
	let wsChild: ChildProcess | undefined
	let projectDir: string
	let globalDir: string
	const prevAgentDir = process.env.PI_CODING_AGENT_DIR

	beforeEach(() => {
		projectDir = mkdtempSync(join(tmpdir(), "acp-ws-project-"))
		globalDir = mkdtempSync(join(tmpdir(), "acp-ws-global-"))
		process.env.PI_CODING_AGENT_DIR = globalDir
		resetAgentCommsIpcForTest()
	})

	afterEach(async () => {
		if (wsChild?.exitCode === null) {
			wsChild.kill("SIGTERM")
			await new Promise((r) => setTimeout(r, 200))
			if (wsChild.exitCode === null) wsChild.kill("SIGKILL")
		}
		wsChild = undefined
		rmSync(projectDir, { recursive: true, force: true })
		rmSync(globalDir, { recursive: true, force: true })
		if (prevAgentDir === undefined) delete process.env.PI_CODING_AGENT_DIR
		else process.env.PI_CODING_AGENT_DIR = prevAgentDir
		resetAgentCommsIpcForTest()
	})

	async function startFixtureAndWriteConfig(): Promise<void> {
		const started = await startWsFixture()
		wsChild = started.child
		mkdirSync(join(projectDir, ".kimchi"), { recursive: true })
		writeFileSync(
			join(projectDir, ".kimchi", "acp-agents.json"),
			JSON.stringify({
				agent_servers: {
					fakews: { transport: "ws", url: `ws://127.0.0.1:${started.port}` },
				},
			}),
		)
	}

	function makeCtx(): ExtensionContext {
		return { cwd: projectDir } as unknown as ExtensionContext
	}

	it("runs an ACP agent over WebSocket without communication (no shim)", async () => {
		await startFixtureAndWriteConfig()

		const record: AgentRecord = {
			id: "acp-ws-record",
			type: "acp:fakews",
			description: "ws test",
			visibility: "user",
			status: "running",
			acp: { server: "fakews" },
			toolUses: 0,
			startedAt: Date.now(),
			currentAttemptId: 0,
			lifetimeUsage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
			compactionCount: 0,
		}

		const result = await runAcpAgent(record, "hello", { description: "test" }, makeCtx())

		expect(result.session).toBeUndefined()
		expect(result.turnsUsed).toBe(1)
		expect(result.aborted).toBe(false)
		expect(result.responseText).toContain("ACP-WS-AGENT: connected over WebSocket")
		// No communication scope → no comms shim → the fixture skips the board post.
		// The WS + shim + board path needs the real binary (getAgentInvocation
		// respawns the vitest entry under unit tests) — covered by the
		// acp-subagent e2e instead.
		expect(result.responseText).toContain("skipped (no comms shim")
	}, 30_000)
})
