import { spawnSync } from "node:child_process"
import { createHash } from "node:crypto"
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import { afterEach, describe, expect, it } from "vitest"
import { createMcpFixture, type McpFixture, seedMcpStdioFixture } from "../tui/support/mcp-fixture.js"

const REPO_ROOT = fileURLToPath(new URL("../../../", import.meta.url))
const BINARY_PATH = resolve(REPO_ROOT, "dist/bin/kimchi")
const PACKAGE_DIR = resolve(REPO_ROOT, "dist/share/kimchi")

function keyringCredentialPath(keyringDir: string, serverName: string): string {
	const account = `sha256-${createHash("sha256").update(serverName, "utf8").digest("hex")}`
	const key = createHash("sha256").update(`pi-mcp-adapter.oauth\0${account}`, "utf8").digest("hex")
	return join(keyringDir, key)
}

describe("compiled kimchi mcp probe command", () => {
	const tempDirs: string[] = []
	const fixtures: McpFixture[] = []

	afterEach(async () => {
		await Promise.all(fixtures.splice(0).map((fixture) => fixture.stop().catch(() => {})))
		for (const dir of tempDirs.splice(0)) rmSync(dir, { recursive: true, force: true })
	})

	it("discovers tools through a real stdio MCP process and emits JSON", async () => {
		const homeDir = mkdtempSync(join(tmpdir(), "kimchi-mcp-probe-home-"))
		const workDir = mkdtempSync(join(tmpdir(), "kimchi-mcp-probe-work-"))
		tempDirs.push(homeDir, workDir)
		const agentDir = join(homeDir, ".config", "kimchi", "harness")
		mkdirSync(agentDir, { recursive: true })
		const fixture = seedMcpStdioFixture(agentDir)
		fixtures.push(fixture)
		const isolatedEnv = Object.fromEntries(
			Object.entries(process.env).filter(([name]) => name !== "NODE_CHANNEL_FD" && name !== "NODE_UNIQUE_ID"),
		)

		const result = spawnSync(BINARY_PATH, ["mcp", "probe", "--json"], {
			cwd: workDir,
			input: JSON.stringify({ name: "probe-fixture", server: fixture.serverDefinition }),
			encoding: "utf-8",
			env: {
				...isolatedEnv,
				HOME: homeDir,
				PI_PACKAGE_DIR: PACKAGE_DIR,
				KIMCHI_NO_UPDATE_CHECK: "1",
			},
			timeout: 30_000,
		})

		expect(result.error).toBeUndefined()
		expect(result.status, result.stderr).toBe(0)
		const output = JSON.parse(result.stdout) as {
			tools: Array<{ name: string; description?: string }>
			needsAuth: boolean
			error: string | null
		}
		expect(output.needsAuth).toBe(false)
		expect(output.error).toBeNull()
		expect(output.tools).toEqual(
			expect.arrayContaining([
				expect.objectContaining({ name: "echo" }),
				expect.objectContaining({ name: "mixed_content" }),
			]),
		)
		expect(fixture.hasEvent("initialized")).toBe(true)
		expect(fixture.hasEvent("tools_listed")).toBe(true)
		expect(fixture.hasEvent("process_exited", { code: 0 })).toBe(true)
	})

	it("preserves configured same-name credentials for a different URL and removes the probe entry", async () => {
		const homeDir = mkdtempSync(join(tmpdir(), "kimchi-mcp-probe-home-"))
		const workDir = mkdtempSync(join(tmpdir(), "kimchi-mcp-probe-work-"))
		tempDirs.push(homeDir, workDir)
		const agentDir = join(homeDir, ".config", "kimchi", "harness")
		mkdirSync(agentDir, { recursive: true })
		const fixture = await createMcpFixture(agentDir, { transport: "oauth" })
		fixtures.push(fixture)

		const serverName = "edited-server"
		const originalServerUrl = "https://original.example.test/mcp"
		writeFileSync(
			join(workDir, ".mcp.json"),
			JSON.stringify({ mcpServers: { [serverName]: { url: originalServerUrl, auth: "oauth" } } }),
		)
		const keyringDir = join(agentDir, "mcp-keyring")
		const credentialPath = keyringCredentialPath(keyringDir, serverName)
		const originalCredential = JSON.stringify({
			tokens: { accessToken: "original-server-token", expiresAt: 2_000_000_000 },
			serverUrl: originalServerUrl,
		})
		mkdirSync(keyringDir, { recursive: true })
		writeFileSync(credentialPath, originalCredential, { encoding: "utf8", mode: 0o600 })

		const isolatedEnv = Object.fromEntries(
			Object.entries(process.env).filter(([name]) => name !== "NODE_CHANNEL_FD" && name !== "NODE_UNIQUE_ID"),
		)
		const result = spawnSync(BINARY_PATH, ["mcp", "probe", "--json"], {
			cwd: workDir,
			input: JSON.stringify({ name: serverName, server: fixture.serverDefinition }),
			encoding: "utf-8",
			env: {
				...isolatedEnv,
				...fixture.env,
				HOME: homeDir,
				PI_PACKAGE_DIR: PACKAGE_DIR,
				KIMCHI_NO_UPDATE_CHECK: "1",
			},
			timeout: 90_000,
		})

		expect(result.error).toBeUndefined()
		expect(result.status, result.stderr).toBe(0)
		expect(readFileSync(credentialPath, "utf8")).toBe(originalCredential)
		expect(readdirSync(keyringDir)).toHaveLength(1)
	})
})
