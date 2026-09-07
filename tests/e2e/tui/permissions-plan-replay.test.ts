import { readFileSync, realpathSync, renameSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import type { CustomEntry } from "@earendil-works/pi-coding-agent"
import { expect, Key, test } from "@microsoft/tui-test"
import type { FermentV2JournalEntry } from "../../../src/extensions/ferment-v2/types.js"
import { STARTUP_TIMEOUT_MS, STREAM_TIMEOUT_MS, viewText, waitForText } from "./support/assertions.js"
import {
	createKimchiFixture,
	createKimchiSessionController,
	stopKimchi,
	TUI_TEST_CONFIG,
	writeTuiArtifact,
} from "./support/kimchi-fixture.js"

test.use(TUI_TEST_CONFIG)

test("Plan mode survives restart without another assistant turn", async ({ terminal }) => {
	const fixture = await createKimchiFixture({ responses: [{ stream: ["REPLAY_SESSION_READY"] }] })
	const sessionFile = join(fixture.workDir, "mode-replay.jsonl")
	const session = createKimchiSessionController(terminal, fixture, {
		extraArgs: ["--session", sessionFile],
		extraEnv: { ...fixture.seedEnv, KIMCHI_PERMISSIONS: "" },
	})
	const steps = []
	try {
		await session.start()
		await session.turn("Establish this session", "REPLAY_SESSION_READY")
		terminal.submit("/permissions mode plan")
		await waitForText(terminal, /plan(?: → shift\+tab)? · basic\b/, { full: false })
		steps.push({
			label: "user selected Plan without another model turn",
			at: new Date().toISOString(),
			view: viewText(terminal),
		})
		const beforeRestart = readEntries(sessionFile)
		expect(
			beforeRestart.filter((entry) => entry.type === "custom" && entry.customType === "permission_mode").at(-1)?.data,
		).toMatchObject({
			mode: "plan",
			initiatedBy: "user",
		})
		const requestCount = fixture.fake.requests.filter((entry) => entry.url === "/openai/v1/chat/completions").length
		await session.restart()
		await waitForText(terminal, /plan(?: → shift\+tab)? · basic\b/, { timeoutMs: STARTUP_TIMEOUT_MS, full: false })
		expect(fixture.fake.requests.filter((entry) => entry.url === "/openai/v1/chat/completions")).toHaveLength(
			requestCount,
		)
		steps.push({
			label: "fresh process restored Plan without inference",
			at: new Date().toISOString(),
			view: viewText(terminal),
		})
		await writeTuiArtifact({ name: "permission-mode-immediate-replay", outcome: "pass", terminal, fixture, steps })
	} catch (error) {
		await writeTuiArtifact({
			name: "permission-mode-immediate-replay",
			outcome: "fail",
			terminal,
			fixture,
			steps,
			error,
		})
		throw error
	} finally {
		await session.quit().catch(() => {})
		await stopKimchi(terminal).catch(() => {})
		await fixture.stop()
	}
})

for (const referenceState of ["changed", "missing"] as const) {
	test(`approved Markdown survives a ${referenceState} saved copy and paused restart`, async ({ terminal }) => {
		const plan = "# Approved Snapshot\n\n## Goal\nReturn exactly APPROVED_TOKEN, with no other text.\n\n"
		const blockedResponse = {
			toolCalls: [
				{
					id: "block",
					function: {
						name: "update_ferment_v2",
						arguments: JSON.stringify({ status: "blocked", reason: "Snapshot inspected." }),
					},
				},
			],
		}
		const fixture = await createKimchiFixture({
			gitInit: true,
			seedHome: (homeDir) => {
				const settingsPath = join(homeDir, ".config", "kimchi", "harness", "settings.json")
				const settings = JSON.parse(readFileSync(settingsPath, "utf8"))
				writeFileSync(
					settingsPath,
					JSON.stringify({
						...settings,
						multiModel: false,
						resources: { ...settings.resources, "extensions.ferment-v2": true },
					}),
				)
			},
			responses: [
				{
					stream: [plan],
					toolCalls: [{ id: "submit", function: { name: "submit_plan", arguments: JSON.stringify({ plan }) } }],
				},
				blockedResponse,
				{
					stream: ["RESUMED_APPROVED_SNAPSHOT"],
					toolCalls: [{ ...blockedResponse.toolCalls[0], id: "block-resumed" }],
				},
			],
		})
		const sessionFile = join(fixture.workDir, "approved-replay.jsonl")
		const session = createKimchiSessionController(terminal, fixture, {
			extraArgs: ["--session", sessionFile],
			extraEnv: { ...fixture.seedEnv, KIMCHI_PERMISSIONS: "" },
		})
		const steps = []
		try {
			await session.start()
			terminal.submit("/permissions mode plan")
			await waitForText(terminal, /plan(?: → shift\+tab)? · basic\b/, { full: false })
			terminal.submit("Prepare the approved snapshot")
			await waitForText(terminal, "Execute the plan", { timeoutMs: STREAM_TIMEOUT_MS })
			const planPath = join(realpathSync(fixture.workDir), ".kimchi", "plans", "approved-snapshot.md")
			expect(readFileSync(planPath, "utf8")).toBe(plan)
			if (referenceState === "changed") writeFileSync(planPath, "# Unapproved\nReturn CHANGED_TOKEN.\n")
			else renameSync(planPath, `${planPath}.saved`)
			steps.push({
				label: `saved copy ${referenceState} before native approval`,
				at: new Date().toISOString(),
				view: viewText(terminal),
			})
			terminal.keyPress(Key.Enter)
			await waitForText(terminal, "Plan execution blocked.", { timeoutMs: STREAM_TIMEOUT_MS })
			const approved = lastRun(sessionFile)
			expect(approved?.objective).toContain(plan)
			expect(approved?.objective).toContain(`Saved plan copy (reference only): ${JSON.stringify(planPath)}`)
			expect(approved?.objective).not.toContain("CHANGED_TOKEN")
			expect(approved?.presentation).toMatchObject({ kind: "approved-plan", planPath })

			terminal.submit("/ferment-v2 pause")
			await waitForText(terminal, "Plan execution paused.", { full: false })
			if (referenceState === "changed") renameSync(planPath, `${planPath}.changed`)
			else writeFileSync(planPath, "# Unapproved\nReturn REPLACED_TOKEN.\n")
			const requestCount = fixture.fake.requests.filter((entry) => entry.url === "/openai/v1/chat/completions").length
			await session.restart()
			await waitForText(terminal, "Plan execution: paused", { timeoutMs: STARTUP_TIMEOUT_MS, full: false })
			expect(lastRun(sessionFile)).toMatchObject({
				id: approved?.id,
				revision: 1,
				objective: approved?.objective,
				status: "paused",
			})
			expect(fixture.fake.requests.filter((entry) => entry.url === "/openai/v1/chat/completions")).toHaveLength(
				requestCount,
			)
			steps.push({
				label: "paused restart retained exact approved objective despite changed reference",
				at: new Date().toISOString(),
				view: viewText(terminal),
			})

			terminal.submit("/ferment-v2 resume")
			await waitForText(terminal, "RESUMED_APPROVED_SNAPSHOT", { timeoutMs: STREAM_TIMEOUT_MS, full: false })
			await waitForText(terminal, "Plan execution blocked.", { timeoutMs: STREAM_TIMEOUT_MS, full: false })
			const request = fixture.fake.requests
				.filter((entry) => entry.url.startsWith("/openai/v1/chat/completions"))
				.at(-1)
			const context = strings(request?.body).find((value) => value.includes("<kimchi_session_ferment_v2>"))
			expect(context).toContain(JSON.stringify(approved?.objective))
			expect(lastRun(sessionFile)).toMatchObject({
				id: approved?.id,
				revision: 1,
				objective: approved?.objective,
				status: "blocked",
			})
			steps.push({
				label: "explicit resume sent approved requirements, not saved-copy contents",
				at: new Date().toISOString(),
				view: viewText(terminal),
			})
			await writeTuiArtifact({
				name: `approved-plan-${referenceState}-replay`,
				outcome: "pass",
				terminal,
				fixture,
				steps,
			})
		} catch (error) {
			await writeTuiArtifact({
				name: `approved-plan-${referenceState}-replay`,
				outcome: "fail",
				terminal,
				fixture,
				steps,
				error,
			})
			throw error
		} finally {
			await session.quit().catch(() => {})
			await stopKimchi(terminal).catch(() => {})
			await fixture.stop()
		}
	})
}

function readEntries<T = unknown>(path: string): CustomEntry<T>[] {
	return readFileSync(path, "utf8")
		.trim()
		.split("\n")
		.map((line) => JSON.parse(line))
		.filter((entry) => entry.type === "custom")
}

function lastRun(path: string) {
	const entry = readEntries<FermentV2JournalEntry>(path).findLast(
		(entry) => entry.customType === "kimchi_ferment_v2_state",
	)
	return entry?.data.op === "put" ? entry.data.fermentV2 : undefined
}

function strings(value: unknown): string[] {
	if (typeof value === "string") return [value]
	if (Array.isArray(value)) return value.flatMap(strings)
	if (value && typeof value === "object") return Object.values(value).flatMap(strings)
	return []
}
