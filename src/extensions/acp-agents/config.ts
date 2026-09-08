/**
 * config.ts — ACP agent server configuration discovery.
 *
 * Hierarchy (project overrides global per server name, same spirit as the
 * agents extension's settings/custom-agents files):
 *   1. Global:  $KIMCHI_CODING_AGENT_DIR/acp-agents.json (default: ~/.config/kimchi/harness/)
 *   2. Project: <cwd>/.kimchi/acp-agents.json (highest)
 *
 * Shape:
 * {
 *   "agent_servers": {
 *     "gemini": {
 *       "transport": "stdio",
 *       "command": "gemini",
 *       "args": ["--acp"],
 *       "displayName": "Gemini",
 *       "default_model": "gemini-2.5-pro",
 *       "permissions": "deny"
 *     }
 *   }
 * }
 *
 * Invalid entries are skipped with a stderr warning — never fatal.
 */

import { existsSync, readFileSync } from "node:fs"
import { join } from "node:path"
import { getAgentDir } from "@earendil-works/pi-coding-agent"

export const ACP_TYPE_PREFIX = "acp:"

/** Subagent type name for an ACP agent server (`acp:<name>`). */
export function acpTypeName(name: string): string {
	return `${ACP_TYPE_PREFIX}${name}`
}

/** Server name for an `acp:<name>` type, or undefined when the type is not ACP-prefixed. */
export function acpServerFromType(type: string): string | undefined {
	return type.startsWith(ACP_TYPE_PREFIX) ? type.slice(ACP_TYPE_PREFIX.length) : undefined
}

export type AcpTransport = "stdio" | "ws"

export interface AcpAgentServerConfig {
	/** Server name as configured (the `acp:` prefix is added for the type name). */
	name: string
	transport: AcpTransport
	/** stdio: executable to spawn. */
	command?: string
	/** stdio: arguments (e.g. ["--acp"]). */
	args?: string[]
	/** stdio: extra environment variables. */
	env?: Record<string, string>
	/** stdio: working directory override. */
	cwd?: string
	/** ws: WebSocket base URL. */
	url?: string
	/** ws: session name on the endpoint (defaults to derived at spawn). */
	sessionName?: string
	/** ws: bearer/connect token. */
	token?: string
	displayName?: string
	defaultModel?: string
	/** requestPermission posture. "allow" is stdio-only — ws is always deny. */
	permissions: "deny" | "allow"
}

function optionalString(val: unknown): string | undefined {
	return typeof val === "string" && val.trim() ? val.trim() : undefined
}

function stringArray(val: unknown): string[] | undefined {
	if (!Array.isArray(val) || !val.every((v) => typeof v === "string")) return undefined
	return val.length > 0 ? val : undefined
}

function stringRecord(val: unknown): Record<string, string> | undefined {
	if (!val || typeof val !== "object" || Array.isArray(val)) return undefined
	const out: Record<string, string> = {}
	for (const [k, v] of Object.entries(val)) {
		if (typeof v !== "string") return undefined
		out[k] = v
	}
	return Object.keys(out).length > 0 ? out : undefined
}

/**
 * Validate one `agent_servers` entry. Returns undefined (with a warning)
 * when the entry cannot produce a spawnable agent server:
 * - name must be a non-empty string without ":" (it becomes `acp:<name>`)
 * - stdio requires `command`
 * - ws requires `url`
 * - `permissions: "allow"` is rejected for ws (the WS client cannot allow)
 */
function sanitizeServerEntry(name: string, raw: unknown): AcpAgentServerConfig | undefined {
	const trimmedName = typeof name === "string" ? name.trim() : ""
	if (!trimmedName || trimmedName.includes(":")) return undefined
	if (!raw || typeof raw !== "object") return undefined
	const r = raw as Record<string, unknown>

	const transport: AcpTransport = r.transport === "ws" ? "ws" : "stdio"
	const permissions: "deny" | "allow" = r.permissions === "allow" ? "allow" : "deny"
	const displayName = optionalString(r.displayName)
	const defaultModel = optionalString(r.default_model) ?? optionalString(r.defaultModel)

	if (transport === "stdio") {
		const command = optionalString(r.command)
		if (!command) return undefined
		return {
			name: trimmedName,
			transport,
			command,
			args: stringArray(r.args),
			env: stringRecord(r.env),
			cwd: optionalString(r.cwd),
			displayName,
			defaultModel,
			permissions,
		}
	}

	const url = optionalString(r.url)
	if (!url) return undefined
	if (permissions === "allow") return undefined
	return {
		name: trimmedName,
		transport: "ws",
		url,
		sessionName: optionalString(r.sessionName) ?? optionalString(r.session_name),
		token: optionalString(r.token),
		displayName,
		defaultModel,
		permissions: "deny",
	}
}

/** Read one config file. Missing file is silent; malformed JSON warns and yields nothing. */
function readServerFile(path: string): Map<string, AcpAgentServerConfig> {
	const out = new Map<string, AcpAgentServerConfig>()
	if (!existsSync(path)) return out
	let parsed: unknown
	try {
		parsed = JSON.parse(readFileSync(path, "utf-8"))
	} catch (err) {
		const reason = err instanceof Error ? err.message : String(err)
		console.warn(`[kimchi-acp-agents] Ignoring malformed config at ${path}: ${reason}`)
		return out
	}
	if (!parsed || typeof parsed !== "object") return out
	const servers = (parsed as { agent_servers?: unknown }).agent_servers
	if (!servers || typeof servers !== "object") return out

	for (const [name, raw] of Object.entries(servers)) {
		const server = sanitizeServerEntry(name, raw)
		if (!server) {
			console.warn(`[kimchi-acp-agents] Skipping invalid agent server "${name}" at ${path}`)
			continue
		}
		out.set(server.name, server)
	}
	return out
}

export function globalConfigPath(): string {
	return join(getAgentDir(), "acp-agents.json")
}

function projectConfigPath(cwd: string): string {
	return join(cwd, ".kimchi", "acp-agents.json")
}

/** Load merged ACP agent servers: global provides defaults, project overrides per name. */
export function loadAcpAgentServers(cwd: string = process.cwd()): Map<string, AcpAgentServerConfig> {
	const merged = new Map<string, AcpAgentServerConfig>()
	for (const [name, server] of readServerFile(globalConfigPath())) merged.set(name, server)
	for (const [name, server] of readServerFile(projectConfigPath(cwd))) merged.set(name, server)
	return merged
}
