import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { getAvailableTypes, registerAgents, resolveType, setAcpAgents } from "../agents/personas/agent-types.js"
import { withExperimentalFeatures } from "../experimental.js"
import { refreshAcpAgents } from "./registry.js"

describe("ACP registry hot-reload", () => {
	let projectDir: string
	let globalDir: string
	const prevAgentDir = process.env.PI_CODING_AGENT_DIR

	beforeEach(() => {
		projectDir = mkdtempSync(join(tmpdir(), "acp-registry-project-"))
		globalDir = mkdtempSync(join(tmpdir(), "acp-registry-global-"))
		process.env.PI_CODING_AGENT_DIR = globalDir
	})

	afterEach(() => {
		rmSync(projectDir, { recursive: true, force: true })
		rmSync(globalDir, { recursive: true, force: true })
		if (prevAgentDir === undefined) delete process.env.PI_CODING_AGENT_DIR
		else process.env.PI_CODING_AGENT_DIR = prevAgentDir
		setAcpAgents(new Map())
		registerAgents(new Map())
	})

	function writeProjectConfig(servers: Record<string, unknown>): void {
		mkdirSync(join(projectDir, ".kimchi"), { recursive: true })
		writeFileSync(join(projectDir, ".kimchi", "acp-agents.json"), JSON.stringify({ agent_servers: servers }))
	}

	it("picks up a server configured after a previous refresh (mid-session config edit)", async () => {
		await withExperimentalFeatures(true, async () => {
			// Session start: no config — nothing registered.
			refreshAcpAgents(projectDir)
			registerAgents(new Map())
			expect(resolveType("acp:late")).toBeUndefined()

			// The user adds a server mid-session; the next Agent-tool reload
			// refreshes ACP entries before re-merging the registry.
			writeProjectConfig({ late: { command: "late-bin" } })
			refreshAcpAgents(projectDir)
			registerAgents(new Map())

			expect(resolveType("acp:late")).toBe("acp:late")
			expect(getAvailableTypes()).toContain("acp:late")
		})
	})

	it("drops a removed server on the next refresh", async () => {
		await withExperimentalFeatures(true, async () => {
			writeProjectConfig({ gone: { command: "x" }, kept: { command: "y" } })
			refreshAcpAgents(projectDir)
			registerAgents(new Map())
			expect(resolveType("acp:gone")).toBe("acp:gone")

			// The config is edited down to one server.
			writeProjectConfig({ kept: { command: "y" } })
			refreshAcpAgents(projectDir)
			registerAgents(new Map())

			expect(resolveType("acp:gone")).toBeUndefined()
			expect(resolveType("acp:kept")).toBe("acp:kept")
		})
	})

	it("no-ops when the experimental flag is off", async () => {
		writeProjectConfig({ off: { command: "x" } })

		await withExperimentalFeatures(false, () => {
			const refreshed = refreshAcpAgents(projectDir)
			expect(refreshed.size).toBe(0)
		})

		// The registry was not touched by the off-state refresh.
		registerAgents(new Map())
		expect(getAvailableTypes().some((t) => t.startsWith("acp:"))).toBe(false)
	})
})
