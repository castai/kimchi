import { createHash } from "node:crypto"
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { afterEach, describe, expect, it, vi } from "vitest"
import { configureMcpKeyringRecoveryHelper, inspectMcpCredentialAccount } from "./keyring-require-bridge.js"

const RECOVERY_HELPER_ENV = "PI_MCP_ADAPTER_KEYRING_RECOVERY_HELPER"

function credentialAccount(serverName: string): string {
	return `sha256-${createHash("sha256").update(serverName, "utf8").digest("hex")}`
}

function credentialPath(keyringDir: string, account: string): string {
	return join(keyringDir, createHash("sha256").update(`pi-mcp-adapter.oauth\0${account}`, "utf8").digest("hex"))
}

describe("configureMcpKeyringRecoveryHelper", () => {
	const tempDirs: string[] = []

	afterEach(() => {
		vi.unstubAllEnvs()
		for (const dir of tempDirs.splice(0)) rmSync(dir, { recursive: true, force: true })
	})

	it("points the adapter at the materialized helper in the auxiliary directory", () => {
		const packageDir = mkdtempSync(join(tmpdir(), "kimchi-keyring-helper-"))
		tempDirs.push(packageDir)
		const helperPath = join(packageDir, "mcp-keyring", "mcp-keyring-helper.cjs")
		mkdirSync(join(packageDir, "mcp-keyring"), { recursive: true })
		writeFileSync(helperPath, "module.exports = {}")
		vi.stubEnv("PI_PACKAGE_DIR", packageDir)
		vi.stubEnv(RECOVERY_HELPER_ENV, "")

		configureMcpKeyringRecoveryHelper()

		expect(existsSync(helperPath)).toBe(true)
		expect(process.env[RECOVERY_HELPER_ENV]).toBe(helperPath)
	})

	it("preserves an explicit adapter helper override", () => {
		vi.stubEnv("PI_PACKAGE_DIR", "/package")
		vi.stubEnv(RECOVERY_HELPER_ENV, "/custom/helper.cjs")

		configureMcpKeyringRecoveryHelper()

		expect(process.env[RECOVERY_HELPER_ENV]).toBe("/custom/helper.cjs")
	})

	it("does not configure a helper path that is not materialized", () => {
		const packageDir = mkdtempSync(join(tmpdir(), "kimchi-keyring-helper-"))
		tempDirs.push(packageDir)
		vi.stubEnv("PI_PACKAGE_DIR", packageDir)
		vi.stubEnv(RECOVERY_HELPER_ENV, "")

		configureMcpKeyringRecoveryHelper()

		expect(process.env[RECOVERY_HELPER_ENV]).toBe("")
	})

	it("distinguishes an empty account from a secure credential entry", () => {
		const keyringDir = mkdtempSync(join(tmpdir(), "kimchi-keyring-store-"))
		tempDirs.push(keyringDir)
		vi.stubEnv("KIMCHI_MCP_E2E_KEYRING_DIR", keyringDir)
		const serverName = "orphaned-server"
		const account = credentialAccount(serverName)
		const entryPath = credentialPath(keyringDir, account)

		expect(inspectMcpCredentialAccount(serverName)).toEqual({ status: "absent" })
		writeFileSync(entryPath, JSON.stringify({ serverUrl: "https://old.example.test/mcp" }))
		expect(inspectMcpCredentialAccount(serverName)).toEqual({
			status: "present",
			serverUrl: "https://old.example.test/mcp",
		})
	})

	it("reads the server URL from a chunked secure credential entry", () => {
		const keyringDir = mkdtempSync(join(tmpdir(), "kimchi-keyring-store-"))
		tempDirs.push(keyringDir)
		vi.stubEnv("KIMCHI_MCP_E2E_KEYRING_DIR", keyringDir)
		const serverName = "chunked-server"
		const account = credentialAccount(serverName)
		const digest = "0123456789abcdef"
		const payload = JSON.stringify({
			serverUrl: "https://chunked.example.test/mcp",
			tokens: { accessToken: "x".repeat(2_000) },
		})
		const midpoint = Math.ceil(payload.length / 2)
		writeFileSync(
			credentialPath(keyringDir, account),
			JSON.stringify({ __piMcpAdapterOAuthChunked: 1, chunkCount: 2, chunkDigest: digest }),
		)
		writeFileSync(credentialPath(keyringDir, `${account}.chunk.${digest}.0`), payload.slice(0, midpoint))
		writeFileSync(credentialPath(keyringDir, `${account}.chunk.${digest}.1`), payload.slice(midpoint))

		expect(inspectMcpCredentialAccount(serverName)).toEqual({
			status: "present",
			serverUrl: "https://chunked.example.test/mcp",
		})
	})

	it("detects a legacy plaintext credential entry", () => {
		const oauthDir = mkdtempSync(join(tmpdir(), "kimchi-oauth-store-"))
		tempDirs.push(oauthDir)
		vi.stubEnv("KIMCHI_MCP_E2E_KEYRING_DIR", join(oauthDir, "empty-keyring"))
		vi.stubEnv("MCP_OAUTH_DIR", oauthDir)
		const serverName = "legacy-server"
		const account = credentialAccount(serverName)
		mkdirSync(join(oauthDir, account), { recursive: true })
		writeFileSync(join(oauthDir, account, "tokens.json"), JSON.stringify({ serverUrl: "https://old.example.test" }))

		expect(inspectMcpCredentialAccount(serverName)).toEqual({
			status: "present",
			serverUrl: "https://old.example.test",
		})
	})
})
