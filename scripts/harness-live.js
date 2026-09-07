#!/usr/bin/env node
// Development-only TUI controller for humans and agents. Not a test runner or CI entrypoint.
import { execFileSync, spawnSync } from "node:child_process"
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { basename, join, resolve } from "node:path"
import { setTimeout } from "node:timers/promises"
import { fileURLToPath } from "node:url"

const repo = fileURLToPath(new URL("../", import.meta.url))
const binary = join(repo, "dist/bin/kimchi")
const [action, target, ...text] = process.argv.slice(2)
const tmux = (...args) => execFileSync("tmux", args, { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] })
const quote = (value) => `'${value.replaceAll("'", "'\\''")}'`
const KEYS = [
	"Enter",
	"Escape",
	"Tab",
	"BTab",
	"Up",
	"Down",
	"Left",
	"Right",
	"Home",
	"End",
	"BSpace",
	"DC",
	"Space",
	"PPage",
	"NPage",
	"C-c",
	"C-p",
	"C-u",
	"C-o",
	"F7",
]

function readRun(path) {
	const dir = resolve(path)
	const run = JSON.parse(readFileSync(join(dir, "live-run.json"), "utf8"))
	if (run.directory !== dir || !/^harness-live-[a-zA-Z0-9-]+$/.test(run.tmux))
		throw new Error("Invalid live-run manifest")
	return run
}

function latestSession(run) {
	const dir = join(run.directory, "sessions")
	return readdirSync(dir)
		.filter((name) => name.endsWith(".jsonl"))
		.filter((name) => {
			const header = readEntries(join(dir, name))[0]
			return header?.type === "session" && !header.parentSession
		})
		.sort()
		.at(-1)
}

function readEntries(path) {
	return readFileSync(path, "utf8")
		.split("\n")
		.flatMap((line) => {
			try {
				return [JSON.parse(line)]
			} catch {
				return []
			} // A live writer may be midway through the last line.
		})
}

function launch(run, resume) {
	if (spawnSync("tmux", ["has-session", "-t", `=${run.tmux}`]).status === 0)
		throw new Error("Run is already live; use status or attach")
	const session = resume ? latestSession(run) : undefined
	if (resume && !session) throw new Error("No saved session to resume")
	const args = [binary, "--session-dir", join(run.directory, "sessions")]
	if (session) args.push("--session", join(run.directory, "sessions", session))
	else args.push("--provider", run.provider, "--model", run.model, "--plan=true")
	const command = `PI_PACKAGE_DIR=${quote(join(repo, "dist/share/kimchi"))} ${args.map(quote).join(" ")}`
	tmux("new-session", "-d", "-s", run.tmux, "-x", "150", "-y", "45", "-c", run.directory, command)
	console.log(`Run: ${run.directory}\nAttach: tmux attach -t =${run.tmux}\nInitial model: ${run.provider}/${run.model}`)
}

function status(run) {
	const live = spawnSync("tmux", ["has-session", "-t", `=${run.tmux}`]).status === 0
	console.log(`Live: ${live} · initial model: ${run.provider}/${run.model} · run: ${run.directory}`)
	if (live) console.log(tmux("capture-pane", "-p", "-t", `=${run.tmux}:`))
	const session = latestSession(run)
	if (!session) return
	console.log(`Session: ${join(run.directory, "sessions", session)}`)
}

try {
	if (action === "start") {
		if (!target || text.length > 1) throw new Error("Usage: start <model> [provider]; provider defaults to kimchi-dev")
		if (!existsSync(binary)) throw new Error("Run pnpm run build:binary first")
		const directory = mkdtempSync(join(tmpdir(), "kimchi-harness-live-"))
		mkdirSync(join(directory, "sessions"))
		execFileSync("git", ["init", "-q", directory])
		const run = {
			directory,
			tmux: `harness-live-${basename(directory)}`,
			model: target,
			provider: text[0] ?? "kimchi-dev",
		}
		writeFileSync(join(directory, "live-run.json"), `${JSON.stringify(run, null, 2)}\n`)
		launch(run, false)
	} else if (["type", "send", "key", "status", "resume", "stop"].includes(action) && target) {
		const run = readRun(target)
		if (action === "status") status(run)
		if (action === "resume") launch(run, true)
		if (action === "type" || action === "send") {
			if (!text.length) throw new Error("Provide the prompt or slash command")
			if (action === "type") {
				tmux("send-keys", "-t", `=${run.tmux}:`, "-l", "--", text.join(" "))
			} else {
				const buffer = `${run.tmux}-input`
				tmux("set-buffer", "-b", buffer, "--", text.join(" "))
				tmux("paste-buffer", "-p", "-d", "-b", buffer, "-t", `=${run.tmux}:`)
				await setTimeout(200) // Let the TUI consume text before submitting the command.
				tmux("send-keys", "-t", `=${run.tmux}:`, "Enter")
			}
		}
		if (action === "key") {
			if (!text.length || text.some((key) => !KEYS.includes(key))) throw new Error(`Keys: ${KEYS.join(" ")}`)
			tmux("send-keys", "-t", `=${run.tmux}:`, ...text)
		}
		if (action === "stop") {
			tmux("kill-session", "-t", `=${run.tmux}`)
			console.log(`Stopped only ${run.tmux}. Artifacts and session remain in ${run.directory}.`)
		}
	} else {
		console.log(
			`Kimchi development controller (manual/agent use only; not CI):\nRequires tmux, a built binary and an existing provider login. No feature resource is required. New sessions start in Plan mode.\n  node scripts/harness-live.js start <model> [provider]\n  node scripts/harness-live.js type <run-dir> '<text without submitting>'\n  node scripts/harness-live.js send <run-dir> '<prompt or /command to submit>'\n  node scripts/harness-live.js key <run-dir> <key> [key ...]\n  node scripts/harness-live.js status <run-dir>\n  node scripts/harness-live.js stop <run-dir>\n  node scripts/harness-live.js resume <run-dir>\nKeys: ${KEYS.join(" ")}\nUse send '/model' to open the model menu (C-p cycles models); navigate with Up/Down, select with Enter, dismiss with Escape.\nUses your existing login/settings. Live calls consume inference credits. Keep prompts scoped to the temporary working directory.`,
		)
	}
} catch (error) {
	console.error(error instanceof Error ? error.message : String(error))
	process.exitCode = 1
}
