import { mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { join } from "node:path"
import { afterEach, beforeEach, expect, it, vi } from "vitest"

const subprocess = vi.hoisted(() => ({ exec: vi.fn(), spawn: vi.fn() }))
vi.mock("node:child_process", () => ({ execFileSync: subprocess.exec, spawnSync: subprocess.spawn }))
vi.mock("node:timers/promises", () => ({ setTimeout: async () => {} }))

let dir: string
let run: { directory: string; tmux: string; pane: string; binary: string; model: string; provider: string }
let createdRuns: string[]

beforeEach(() => {
	dir = mkdtempSync(join(tmpdir(), "kimchi-controller-test-"))
	run = {
		directory: dir,
		tmux: "harness-live-test",
		pane: "%7",
		binary: process.execPath,
		model: "basic",
		provider: "fake",
	}
	createdRuns = []
	mkdirSync(join(dir, "sessions"))
	writeFileSync(join(dir, "live-run.json"), JSON.stringify(run))
	subprocess.spawn.mockReset().mockReturnValue({ status: 1 })
	subprocess.exec.mockReset().mockImplementation((file: string, args: string[]) => {
		if (file === "git") createdRuns.push(args[2])
		if (args[0] === "new-session") {
			run.tmux = args[args.indexOf("-s") + 1]
			return "%7\n"
		}
		if (args[0] === "display-message") return `${run.tmux}\n`
		return ""
	})
	vi.spyOn(console, "log").mockImplementation(() => {})
	vi.spyOn(console, "error").mockImplementation(() => {})
})

afterEach(() => {
	for (const path of [dir, ...createdRuns]) rmSync(path, { recursive: true, force: true })
	vi.restoreAllMocks()
	vi.unstubAllEnvs()
})

async function cli(...args: string[]) {
	const argv = process.argv
	const exitCode = process.exitCode
	process.argv = [process.execPath, "harness-live.mjs", ...args]
	process.exitCode = undefined
	try {
		vi.resetModules()
		await import("./harness-live.mjs")
		return process.exitCode ?? 0
	} finally {
		process.argv = argv
		process.exitCode = exitCode
	}
}

it("saves the selected executable and pane, quoting paths independently of the skill location", async () => {
	const binary = join(dir, "kimchi user's build")
	symlinkSync(process.execPath, binary)
	vi.stubEnv("KIMCHI_BINARY", binary)
	expect(await cli("start", "basic", "fake")).toBe(0)
	const saved = JSON.parse(readFileSync(join(createdRuns[0], "live-run.json"), "utf8"))
	expect(saved).toMatchObject({ binary, pane: "%7" })
	const command = subprocess.exec.mock.calls.find(([, args]) => args[0] === "new-session")?.[1].at(-1)
	expect(command).toContain("unset PI_PACKAGE_DIR; exec '")
	expect(command).toContain("user'\\''s build'")
	expect(command).toContain("'--plan=true'")
})

it("finds installed kimchi on PATH without a development checkout", async () => {
	vi.stubEnv("KIMCHI_BINARY", undefined)
	vi.stubEnv("PATH", dir)
	symlinkSync(process.execPath, join(dir, "kimchi"))
	expect(await cli("start", "basic")).toBe(0)
	expect(JSON.parse(readFileSync(join(createdRuns[0], "live-run.json"), "utf8")).binary).toBe(join(dir, "kimchi"))
})

it("resumes the root session with the saved binary and leaves saved model restoration to Kimchi", async () => {
	writeFileSync(join(dir, "sessions/2026-01-root.jsonl"), '{"type":"session"}\n{"partial":')
	writeFileSync(join(dir, "sessions/2026-02-child.jsonl"), '{"type":"session","parentSession":"root"}\n')
	vi.stubEnv("KIMCHI_BINARY", "/different/kimchi")
	expect(await cli("resume", dir)).toBe(0)
	const command = subprocess.exec.mock.calls.find(([, args]) => args[0] === "new-session")?.[1].at(-1)
	expect(command).toContain(`'${run.binary}'`)
	expect(command).toContain("2026-01-root.jsonl")
	expect(command).not.toContain("2026-02-child.jsonl")
	expect(command).not.toContain("--model")
	expect(command).not.toContain("--plan")
})

it("types and captures the original pane, and stops only the exact session", async () => {
	expect(await cli("type", dir, "/model")).toBe(0)
	expect(subprocess.exec).toHaveBeenCalledWith(
		"tmux",
		["send-keys", "-t", "%7", "-l", "--", "/model"],
		expect.anything(),
	)
	subprocess.spawn.mockReturnValue({ status: 0 })
	expect(await cli("status", dir)).toBe(0)
	expect(subprocess.exec).toHaveBeenCalledWith("tmux", ["capture-pane", "-p", "-t", "%7"], expect.anything())
	expect(await cli("stop", dir)).toBe(0)
	expect(subprocess.exec).toHaveBeenCalledWith("tmux", ["kill-session", "-t", "=harness-live-test"], expect.anything())
})

it("refuses a pane reassigned to another session", async () => {
	subprocess.exec.mockReturnValue("neighbor\n")
	expect(await cli("key", dir, "Enter")).toBe(1)
	expect(subprocess.exec.mock.calls.some(([, args]) => args[0] === "send-keys")).toBe(false)
})

it("refuses multiline type input before any key injection", async () => {
	expect(await cli("type", dir, "first\nsecond")).toBe(1)
	expect(subprocess.exec.mock.calls.some(([, args]) => args[0] === "send-keys")).toBe(false)
})

it("passes large multiline prompts through stdin and cleans up the buffer when paste fails", async () => {
	const input = "prompt line\n".repeat(20000)
	subprocess.exec.mockImplementation((_file: string, args: string[]) => {
		if (args[0] === "display-message") return run.tmux
		if (args[0] === "paste-buffer") throw new Error("pane closed")
		return ""
	})
	expect(await cli("send", dir, input)).toBe(1)
	expect(subprocess.exec).toHaveBeenCalledWith(
		"tmux",
		["load-buffer", "-b", expect.any(String), "-"],
		expect.objectContaining({ input }),
	)
	expect(subprocess.spawn).toHaveBeenCalledWith("tmux", ["delete-buffer", "-b", expect.any(String)], expect.anything())
	expect(subprocess.exec.mock.calls.some(([, args]) => args[0] === "send-keys")).toBe(false)
})

it("reports missing executables and invalid invocations without launching", async () => {
	vi.stubEnv("KIMCHI_BINARY", join(dir, "missing"))
	expect(await cli("start", "basic")).toBe(1)
	expect(await cli("send")).toBe(1)
	expect(await cli("status", dir, "extra")).toBe(1)
	expect(await cli("key", dir, "unknown-key")).toBe(1)
	expect(subprocess.exec).not.toHaveBeenCalled()
})
