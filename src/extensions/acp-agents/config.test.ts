import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { getAvailableTypes, registerAgents, resolveType, setAcpAgents } from "../agents/personas/agent-types.js"
import type { AgentConfig } from "../agents/personas/types.js"
import { acpTypeName, loadAcpAgentServers } from "./config.js"

describe("ACP agent server config", () => {
	let globalDir: string
	let projectDir: string
	const prevAgentDir = process.env.PI_CODING_AGENT_DIR

	beforeEach(() => {
		globalDir = mkdtempSync(join(tmpdir(), "acp-agents-global-"))
		process.env.PI_CODING_AGENT_DIR = globalDir
		projectDir = mkdtempSync(join(tmpdir(), "acp-agents-project-"))
	})

	afterEach(() => {
		rmSync(globalDir, { recursive: true, force: true })
		rmSync(projectDir, { recursive: true, force: true })
		if (prevAgentDir === undefined) delete process.env.PI_CODING_AGENT_DIR
		else process.env.PI_CODING_AGENT_DIR = prevAgentDir
	})

	function writeGlobalConfig(servers: Record<string, unknown>): void {
		writeFileSync(join(globalDir, "acp-agents.json"), JSON.stringify({ agent_servers: servers }))
	}

	function writeProjectConfig(servers: Record<string, unknown>): void {
		mkdirSync(join(projectDir, ".kimchi"), { recursive: true })
		writeFileSync(join(projectDir, ".kimchi", "acp-agents.json"), JSON.stringify({ agent_servers: servers }))
	}

	it("returns no servers when no config files exist", () => {
		expect([...loadAcpAgentServers(projectDir).keys()]).toEqual([])
	})

	it("loads a stdio server with defaults (transport stdio, permissions deny)", () => {
		writeGlobalConfig({ gemini: { command: "gemini", args: ["--acp"] } })

		const servers = loadAcpAgentServers(projectDir)
		expect(servers.size).toBe(1)
		const gemini = servers.get("gemini")
		expect(gemini).toMatchObject({
			name: "gemini",
			transport: "stdio",
			command: "gemini",
			args: ["--acp"],
			permissions: "deny",
		})
	})

	it("loads a ws server with token and sessionName", () => {
		writeGlobalConfig({
			remote: { transport: "ws", url: "wss://example.com", token: "sekrit", sessionName: "acp-1" },
		})

		const remote = loadAcpAgentServers(projectDir).get("remote")
		expect(remote).toMatchObject({
			name: "remote",
			transport: "ws",
			url: "wss://example.com",
			token: "sekrit",
			sessionName: "acp-1",
			permissions: "deny",
		})
	})

	it("project config overrides global per server name", () => {
		writeGlobalConfig({
			gemini: { command: "gemini", args: ["--acp"] },
			other: { command: "other" },
		})
		writeProjectConfig({ gemini: { command: "/custom/gemini" } })

		const servers = loadAcpAgentServers(projectDir)
		expect(servers.get("gemini")?.command).toBe("/custom/gemini")
		expect(servers.get("other")?.command).toBe("other")
	})

	it("skips invalid entries: stdio without command, ws without url, ws with permissions allow, bad names", () => {
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {})
		writeGlobalConfig({
			noCommand: { transport: "stdio" },
			noUrl: { transport: "ws" },
			noAllowOverWs: { transport: "ws", url: "wss://example.com", permissions: "allow" },
			"": { command: "x" },
			"we:ird": { command: "x" },
		})

		const servers = loadAcpAgentServers(projectDir)
		expect([...servers.keys()]).toEqual([])
		// One warning per skipped entry.
		expect(warn).toHaveBeenCalledTimes(5)
		warn.mockRestore()
	})

	it("warns and skips a malformed JSON config file", () => {
		const warn = vi.spyOn(console, "warn").mockImplementation(() => {})
		writeFileSync(join(globalDir, "acp-agents.json"), "{ not json")

		expect([...loadAcpAgentServers(projectDir).keys()]).toEqual([])
		expect(warn).toHaveBeenCalledTimes(1)
		warn.mockRestore()
	})

	it("treats a non-object agent_servers value as empty", () => {
		writeGlobalConfig({})
		writeFileSync(join(globalDir, "acp-agents.json"), JSON.stringify({ agent_servers: ["not", "an", "object"] }))

		expect([...loadAcpAgentServers(projectDir).keys()]).toEqual([])
	})

	it("prefixes type names with acp:", () => {
		expect(acpTypeName("gemini")).toBe("acp:gemini")
	})
})

describe("ACP registry merge", () => {
	afterEach(() => {
		setAcpAgents(new Map())
		registerAgents(new Map())
	})

	function acpConfig(name: string): AgentConfig {
		return {
			name,
			displayName: name,
			description: "External ACP agent",
			builtinToolNames: [],
			extensions: false,
			skills: false,
			systemPrompt: "",
			promptMode: "replace",
			enabled: true,
			source: "acp",
		}
	}

	it("merges ACP entries through registerAgents", () => {
		setAcpAgents(new Map([["acp:gemini", acpConfig("acp:gemini")]]))
		registerAgents(new Map())

		expect(getAvailableTypes()).toContain("acp:gemini")
		expect(resolveType("acp:gemini")).toBe("acp:gemini")
	})

	it("resolves acp: names case-insensitively", () => {
		setAcpAgents(new Map([["acp:gemini", acpConfig("acp:gemini")]]))
		registerAgents(new Map())

		expect(resolveType("ACP:GEMINI")).toBe("acp:gemini")
	})

	it("excludes ACP entries when the map is empty (experimental off)", () => {
		setAcpAgents(new Map())
		registerAgents(new Map())

		expect(getAvailableTypes().some((t) => t.startsWith("acp:"))).toBe(false)
	})

	it("keeps ACP entries across registerAgents reloads", () => {
		setAcpAgents(new Map([["acp:gemini", acpConfig("acp:gemini")]]))
		registerAgents(new Map())
		// A later reload (e.g. reloadCustomAgents) must not drop ACP entries.
		registerAgents(new Map([["Custom", { ...acpConfig("Custom"), source: "project" }]]))

		expect(getAvailableTypes()).toContain("acp:gemini")
		expect(getAvailableTypes()).toContain("Custom")
	})
})
