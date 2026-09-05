import type { ExtensionAPI, ToolDefinition } from "@earendil-works/pi-coding-agent"
import type { McpAdapterOptions } from "pi-mcp-adapter/types"
import { Type } from "typebox"
import { afterAll, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

const upstream = vi.hoisted(() => ({
	options: undefined as McpAdapterOptions | undefined,
	gatewayExecute: vi.fn<ToolDefinition["execute"]>(),
	logout: vi.fn(),
	sessionStart: vi.fn(),
	sessionShutdown: vi.fn(),
}))

vi.mock("pi-mcp-adapter", () => ({
	createMcpAdapter: vi.fn((options: McpAdapterOptions) => (api: ExtensionAPI) => {
		upstream.options = options
		api.on("session_start", async () => {
			await upstream.sessionStart()
		})
		api.on("session_shutdown", async () => {
			await upstream.sessionShutdown()
		})
		api.registerCommand("mcp", {
			description: "MCP",
			handler: async (args, ctx) => upstream.logout(args, ctx),
		})
		api.registerTool({
			name: "mcp",
			label: "MCP",
			description: "MCP gateway",
			parameters: Type.Record(Type.String(), Type.Unknown()),
			execute: upstream.gatewayExecute,
		})
	}),
}))

const installKeyringRequireBridge = vi.hoisted(() => vi.fn())
vi.mock("./keyring-require-bridge.js", () => ({ installKeyringRequireBridge }))

const configuredServers = vi.hoisted(() => ({ value: {} as Record<string, { url?: string }> }))
vi.mock("./config.js", () => ({
	loadKimchiMcpConfig: vi.fn(() => ({ config: { mcpServers: configuredServers.value }, warnings: [] })),
}))

import { inspectMcpOAuthTokensForUrl, updateMcpOAuthTokensForUrl } from "pi-mcp-adapter/oauth"
import { UpstreamMcpProbe } from "./probe.js"

const authStoreEnv = "PI_MCP_ADAPTER_TEST_AUTH_STORE"
let originalAuthStore: string | undefined

function gatewayResult(details: Record<string, unknown>, text = "") {
	return {
		content: text ? [{ type: "text" as const, text }] : [],
		details,
	}
}

function configuredServerNames(): string[] {
	return Object.keys(upstream.options?.config?.mcpServers ?? {})
}

beforeAll(() => {
	originalAuthStore = process.env[authStoreEnv]
	process.env[authStoreEnv] = "memory"
})

afterAll(() => {
	if (originalAuthStore === undefined) delete process.env[authStoreEnv]
	else process.env[authStoreEnv] = originalAuthStore
})

beforeEach(() => {
	vi.clearAllMocks()
	configuredServers.value = {}
	upstream.options = undefined
	upstream.gatewayExecute.mockImplementation(async (_toolCallId, params) => {
		if (typeof params === "object" && params !== null && "connect" in params) {
			return gatewayResult({ tools: [] })
		}
		throw new Error(`Unexpected gateway request: ${JSON.stringify(params)}`)
	})
})

describe("UpstreamMcpProbe", () => {
	it("discovers and describes tools before shutting the adapter down", async () => {
		upstream.gatewayExecute.mockImplementation(async (_toolCallId, params) => {
			if (typeof params !== "object" || params === null) throw new Error("Expected gateway parameters")
			if ("connect" in params) return gatewayResult({ tools: ["lookup", "status", 42] })
			if ("describe" in params && params.describe === "lookup") {
				return gatewayResult({ tool: { description: "Look up a record" } })
			}
			if ("describe" in params && params.describe === "status") return gatewayResult({ tool: {} })
			throw new Error(`Unexpected gateway request: ${JSON.stringify(params)}`)
		})

		const result = await new UpstreamMcpProbe().probeTools(
			"fixture",
			{ command: "node", args: ["server.js"] },
			{ authenticate: true, cwd: "/work" },
		)

		expect(result).toEqual({
			tools: [{ name: "lookup", description: "Look up a record" }, { name: "status" }],
			needsAuth: false,
			error: null,
		})
		expect(upstream.options?.config).toMatchObject({
			mcpServers: {
				fixture: { command: "node", args: ["server.js"], directTools: false, lifecycle: "lazy" },
			},
			settings: { autoAuth: true, directTools: false, scriptMode: false },
		})
		expect(installKeyringRequireBridge).toHaveBeenCalledOnce()
		expect(upstream.sessionStart).toHaveBeenCalledOnce()
		expect(upstream.sessionShutdown).toHaveBeenCalledOnce()
	})

	it.each([
		{ authenticate: false, expectedError: null },
		{ authenticate: true, expectedError: "Authorization required" },
	])("maps auth-required results when authenticate=$authenticate", async ({ authenticate, expectedError }) => {
		upstream.gatewayExecute.mockResolvedValue(
			gatewayResult({ error: "auth_required", message: "Authorization required" }),
		)

		await expect(
			new UpstreamMcpProbe().probeTools(`auth-${authenticate}`, { url: "https://example.test/mcp" }, { authenticate }),
		).resolves.toEqual({ tools: [], needsAuth: true, error: expectedError })
		expect(upstream.sessionShutdown).toHaveBeenCalledOnce()
	})

	it("uses the real server name when credentials match the configured URL", async () => {
		const name = "matching-url"
		const url = "https://example.test/mcp"
		updateMcpOAuthTokensForUrl(name, url, { accessToken: "existing-token" })

		await new UpstreamMcpProbe().probeTools(name, { url })

		expect(inspectMcpOAuthTokensForUrl(name, url).status).toBe("present")
		expect(configuredServerNames()).toEqual([name])
		expect(upstream.logout).not.toHaveBeenCalled()
	})

	it("uses the real server name when no credentials exist", async () => {
		const name = "no-credentials"

		await new UpstreamMcpProbe().probeTools(name, { url: "https://example.test/mcp" })

		expect(configuredServerNames()).toEqual([name])
		expect(upstream.logout).not.toHaveBeenCalled()
	})

	it("uses the real server name when the saved URL has no credentials", async () => {
		const name = "edited-without-credentials"
		configuredServers.value[name] = { url: "https://old.example.test/mcp" }

		await new UpstreamMcpProbe().probeTools(name, { url: "https://new.example.test/mcp" }, { authenticate: true })

		expect(configuredServerNames()).toEqual([name])
		expect(upstream.logout).not.toHaveBeenCalled()
	})

	it("isolates configured credentials when the saved URL differs from the probed URL", async () => {
		const name = "different-url"
		const storedUrl = "https://old.example.test/mcp"
		const probedUrl = "https://new.example.test/mcp"
		configuredServers.value[name] = { url: storedUrl }
		updateMcpOAuthTokensForUrl(name, storedUrl, { accessToken: "preserve-me" })

		await new UpstreamMcpProbe().probeTools(name, { url: probedUrl }, { authenticate: true })

		const [probeName] = configuredServerNames()
		expect(probeName).toMatch(/^__probe_[0-9a-f-]{36}$/)
		expect(upstream.logout).toHaveBeenCalledWith(`logout ${probeName}`, expect.anything())
		expect(inspectMcpOAuthTokensForUrl(name, storedUrl).status).toBe("present")
	})

	it("cleans up an isolated credential entry when probing throws", async () => {
		const name = "different-url-failure"
		const storedUrl = "https://old.example.test/mcp"
		configuredServers.value[name] = { url: storedUrl }
		updateMcpOAuthTokensForUrl(name, storedUrl, { accessToken: "preserve-me" })
		upstream.gatewayExecute.mockRejectedValue(new Error("connect failed"))

		await expect(
			new UpstreamMcpProbe().probeTools(name, { url: "https://new.example.test/mcp" }, { authenticate: true }),
		).rejects.toThrow("connect failed")

		const [probeName] = configuredServerNames()
		expect(upstream.logout).toHaveBeenCalledWith(`logout ${probeName}`, expect.anything())
		expect(upstream.sessionShutdown).toHaveBeenCalledOnce()
		expect(inspectMcpOAuthTokensForUrl(name, storedUrl).status).toBe("present")
	})

	it("shuts the adapter down when probing throws", async () => {
		upstream.gatewayExecute.mockRejectedValue(new Error("connect failed"))

		await expect(new UpstreamMcpProbe().probeTools("failed", { command: "node", args: ["server.js"] })).rejects.toThrow(
			"connect failed",
		)
		expect(upstream.sessionShutdown).toHaveBeenCalledOnce()
	})

	it("shuts the adapter down when a probe is aborted", async () => {
		const controller = new AbortController()
		let markGatewayStarted: () => void = () => {}
		const gatewayStarted = new Promise<void>((resolve) => {
			markGatewayStarted = resolve
		})
		upstream.gatewayExecute.mockImplementation(
			(_toolCallId, _params, signal) =>
				new Promise((_resolve, reject) => {
					markGatewayStarted()
					if (signal?.aborted) {
						reject(signal.reason)
						return
					}
					signal?.addEventListener("abort", () => reject(signal.reason), { once: true })
				}),
		)
		const result = new UpstreamMcpProbe().probeTools(
			"aborted",
			{ command: "node", args: ["server.js"] },
			{ signal: controller.signal },
		)

		await gatewayStarted
		controller.abort(new Error("probe aborted"))

		await expect(result).rejects.toThrow("probe aborted")
		expect(upstream.sessionShutdown).toHaveBeenCalledOnce()
	})
})
