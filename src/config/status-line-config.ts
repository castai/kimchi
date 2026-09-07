import { homedir } from "node:os"
import { join } from "node:path"
import { readJson, writeJson } from "./json.js"

export type StatusLineElementId =
	| "permissions"
	| "model"
	| "thinking"
	| "ferment"
	| "ferment-v2"
	| "agents"
	| "context"
	| "usage"
	| "phase"
	| "tags"
	| "team"
	| "credits"
	| "budget"

export type StatusLineConfig = { pinned: StatusLineElementId[]; lines?: number }

export const MAX_STATUS_LINE_LINES = 3

const STATUS_LINE_KEY = "statusLine"

export const DEFAULT_STATUS_LINE_PINNED: StatusLineElementId[] = ["thinking", "agents", "context", "usage"]

/** All status line elements for the settings UI.
 *  canPin=false marks elements that are always visible and cannot be toggled. */
export const STATUS_LINE_ELEMENTS: Array<{
	id: StatusLineElementId
	label: string
	description: string
	canPin?: boolean
}> = [
	{
		id: "permissions",
		label: "Permissions mode",
		description: "● default / ○ auto  → shift+tab",
		canPin: false,
	},
	{
		id: "model",
		label: "Model",
		description: "Active model or multi-model  → ctrl+p",
		canPin: false,
	},
	{
		id: "thinking",
		label: "Thinking level",
		description: "Current model thinking level",
	},
	{
		id: "ferment",
		label: "Ferment",
		description: "Ferment status & controls",
	},
	{
		id: "agents",
		label: "Agents",
		description: "Active sub-agent count",
	},
	{
		id: "context",
		label: "Context",
		description: "Context usage bar + percentage",
	},
	{
		id: "usage",
		label: "Token I/O",
		description: "Token input (↑) and output (↓)",
	},
	{
		id: "phase",
		label: "Phase",
		description: "Current work phase",
	},
	{
		id: "tags",
		label: "Tags",
		description: "Active tags (env:, region: …)",
	},
	{
		id: "team",
		label: "Team",
		description: "Team tag value",
	},
	{
		id: "credits",
		label: "Credits",
		description: "Remaining credit balance",
	},
	{
		id: "budget",
		label: "Budget",
		description: "Budget usage and limit",
	},
	{
		id: "ferment-v2",
		label: "Ferment V2",
		description: "Objective status (shown automatically while set)",
	},
]

function getSettingsPath(): string {
	return join(homedir(), ".config", "kimchi", "harness", "settings.json")
}

function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}

let _config: StatusLineConfig | null = null

/** Reset the in-memory config cache. Exposed for test isolation only. */
export function _invalidateStatusLineConfigCache(): void {
	_config = null
}

export function readStatusLineConfig(): StatusLineConfig {
	if (_config !== null) return _config
	const settings = readJson(getSettingsPath())
	if (!(STATUS_LINE_KEY in settings)) {
		_config = { pinned: [...DEFAULT_STATUS_LINE_PINNED] }
		return _config
	}
	const raw = asRecord(settings[STATUS_LINE_KEY])
	const values = Array.isArray(raw.pinned) ? raw.pinned : []
	const pinned: StatusLineElementId[] = []
	for (const value of values) {
		if (value === "billing") {
			pinned.push("credits", "budget")
		} else if (STATUS_LINE_ELEMENTS.some((element) => element.id === value)) {
			pinned.push(value as StatusLineElementId)
		}
	}
	_config = { pinned: [...new Set(pinned)], ...(isValidLineCount(raw.lines) ? { lines: raw.lines } : {}) }
	return _config
}

export function writeStatusLineConfig(config: StatusLineConfig): void {
	const path = getSettingsPath()
	const settings = readJson(path)
	settings[STATUS_LINE_KEY] = { ...asRecord(settings[STATUS_LINE_KEY]), ...config }
	writeJson(path, settings)
	_config = { ...config, pinned: [...config.pinned] }
}

export function setStatusLineElementPinned(id: StatusLineElementId, pinned: boolean): void {
	const current = readStatusLineConfig()
	const set = new Set(current.pinned)
	if (pinned) {
		set.add(id)
	} else {
		set.delete(id)
	}
	writeStatusLineConfig({ ...current, pinned: [...set] })
}

function isValidLineCount(value: unknown): value is number {
	return typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= MAX_STATUS_LINE_LINES
}

export function setStatusLineLines(lines: number): void {
	if (!isValidLineCount(lines)) throw new Error(`Status line rows must be between 1 and ${MAX_STATUS_LINE_LINES}.`)
	writeStatusLineConfig({ ...readStatusLineConfig(), lines })
}

export function isStatusLineElementPinned(id: StatusLineElementId): boolean {
	return readStatusLineConfig().pinned.includes(id)
}
