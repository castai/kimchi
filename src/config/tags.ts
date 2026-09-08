import { homedir } from "node:os"
import { join } from "node:path"
import { findNearestAncestorPath } from "../utils/find-nearest-ancestor.js"
import { readJson } from "./json.js"

// ─── Tag format validation ───────────────────────────────────────────────────

const TAG_RE = /^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?:[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$/

export function isValidTag(tag: string): boolean {
	if (!TAG_RE.test(tag)) return false
	const [key, value] = tag.split(":", 2)
	return key.length <= 64 && value.length <= 64
}

export function parseTag(tag: string): { key: string; value: string } | null {
	if (!isValidTag(tag)) return null
	const [key, value] = tag.split(":", 2)
	return { key, value }
}

// ─── Tiered tag defaults ─────────────────────────────────────────────────────

export type TagTier = "env" | "project" | "global"

/** Resolved default tags plus the tier each surviving one came from. */
export interface TagTierDefaults {
	tags: string[]
	tierByTag: Map<string, TagTier>
}

interface TagsConfig {
	tags?: string[]
}

const GLOBAL_TAGS_FILE_REL = join(".config", "kimchi", "tags.json")
const PROJECT_TAGS_FILE_REL = join(".kimchi", "tags.json")

function readTagFile(path: string): string[] {
	try {
		const config = readJson(path) as TagsConfig
		if (!Array.isArray(config.tags)) return []
		return config.tags.filter(isValidTag)
	} catch (err) {
		// Fail open — a broken tag config must not block the session — but
		// surface it rather than silently ignoring it.
		console.warn(`[tags] ignoring unreadable tag config ${path}: ${err}`)
		return []
	}
}

function parseEnvTags(envTags: string): string[] {
	const out: string[] = []
	for (const tag of envTags.split(",")) {
		const trimmed = tag.trim()
		if (isValidTag(trimmed)) out.push(trimmed)
	}
	return out
}

/**
 * Resolve tag defaults from the config hierarchy. Tiers, weakest to strongest:
 *
 *   1. global  — `~/.config/kimchi/tags.json`
 *   2. project — nearest-ancestor `.kimchi/tags.json` from cwd
 *   3. env     — `KIMCHI_TAGS` (comma-separated)
 *
 * Tiers are unioned; when tiers define the same tag key, the stronger tier's
 * value replaces the weaker's. Same-key tags within a single tier coexist
 * (matches the historical flat-union behaviour of file + env). Output is
 * sorted for determinism.
 */
export function resolveDefaultTags(options?: { cwd?: string; homeDir?: string; envTags?: string }): TagTierDefaults {
	const cwd = options?.cwd ?? process.cwd()
	const home = options?.homeDir ?? homedir()
	const envTags = options?.envTags ?? process.env.KIMCHI_TAGS

	const tiers: Array<{ tier: TagTier; tags: string[] }> = [
		{ tier: "global", tags: readTagFile(join(home, GLOBAL_TAGS_FILE_REL)) },
	]
	const projectPath = findNearestAncestorPath(cwd, PROJECT_TAGS_FILE_REL)
	if (projectPath) tiers.push({ tier: "project", tags: readTagFile(projectPath) })
	const envTagList = envTags ? parseEnvTags(envTags) : []
	if (envTagList.length > 0) tiers.push({ tier: "env", tags: envTagList })

	const tierByTag = new Map<string, TagTier>()
	for (const { tier, tags } of tiers) {
		for (const tag of tags) {
			const key = parseTag(tag)?.key
			if (key === undefined) continue
			// A weaker tier's tag with the same key loses to this tier's value.
			// Same-key tags within one tier coexist.
			for (const [existing, existingTier] of tierByTag) {
				if (existingTier !== tier && parseTag(existing)?.key === key) tierByTag.delete(existing)
			}
			tierByTag.set(tag, tier)
		}
	}

	return { tags: [...tierByTag.keys()].sort(), tierByTag }
}
