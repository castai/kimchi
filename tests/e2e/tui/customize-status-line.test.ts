import { readFileSync, writeFileSync } from "node:fs"
import { join } from "node:path"
import { expect, Key, test } from "@microsoft/tui-test"
import { viewText, waitForText } from "./support/assertions.js"
import { runKimchiSession, TUI_TEST_CONFIG } from "./support/kimchi-fixture.js"

test.use({ ...TUI_TEST_CONFIG, columns: 80 })

test("changing status rows and pinning Ferment V2 preserves an existing multi-line script", async ({ terminal }) => {
	await runKimchiSession(
		terminal,
		{
			artifactName: "customize-status-rows-script",
			responses: [],
			seedHome(homeDir, workDir) {
				const command = join(workDir, "status-script.sh")
				writeFileSync(command, "#!/bin/sh\nprintf 'CUSTOM FIRST ROW\\nCUSTOM SECOND ROW\\n'\n", { mode: 0o755 })
				const path = join(homeDir, ".config", "kimchi", "harness", "settings.json")
				const settings = JSON.parse(readFileSync(path, "utf8"))
				settings.statusLine = { pinned: [], command }
				writeFileSync(path, JSON.stringify(settings))
			},
		},
		async (fixture, trace) => {
			await waitForText(terminal, "CUSTOM SECOND ROW", { full: false })
			terminal.submit("/customize-status-line")
			await waitForText(terminal, "Status rows: 1", { full: false })
			// The first selectable field is Thinking; Ferment V2 is the last field.
			for (let index = 0; index < 10; index++) terminal.keyDown()
			terminal.keyPress(Key.Space)
			terminal.keyDown()
			terminal.keyPress(Key.Enter)
			await waitForText(terminal, "Status rows: 2", { full: false })
			trace.step("pinned Ferment V2 and changed native controls to two rows")
			terminal.keyPress(Key.Escape)
			await waitForText(terminal, "Ferment V2: —", { full: false })
			const view = viewText(terminal)
			expect(view).toContain("CUSTOM FIRST ROW")
			expect(view).toContain("CUSTOM SECOND ROW")
			expect(view).toContain("basic")
			const settings = JSON.parse(readFileSync(join(fixture.agentDir, "settings.json"), "utf8"))
			expect(settings.statusLine).toEqual({
				pinned: ["ferment-v2"],
				command: join(fixture.workDir, "status-script.sh"),
				lines: 2,
			})
			trace.step("script output, native controls, row count and V2 pin survived customization")
		},
	)
})

test("the native status rows show a running objective, its blocked state, and clearing it", async ({ terminal }) => {
	await runKimchiSession(
		terminal,
		{
			artifactName: "status-rows-ferment-v2",
			seedHome(homeDir) {
				const path = join(homeDir, ".config", "kimchi", "harness", "settings.json")
				const settings = JSON.parse(readFileSync(path, "utf8"))
				settings.resources = { "extensions.ferment-v2": true }
				settings.statusLine = { pinned: [], lines: 2 }
				writeFileSync(path, JSON.stringify(settings))
			},
			responses: [
				{
					stream: ["Checking the prerequisite."],
					textDelayMs: 1_000,
					toolCalls: [
						{
							id: "block-status-objective",
							function: {
								name: "update_ferment_v2",
								arguments: JSON.stringify({ status: "blocked", reason: "Needs user input." }),
							},
						},
					],
				},
			],
		},
		async (_fixture, trace) => {
			expect(viewText(terminal)).not.toContain("Ferment V2:")
			terminal.submit("/ferment-v2 Cache layer")
			await waitForText(terminal, "Ferment V2: running", { full: false })
			trace.step("running objective appears in native footer")
			await waitForText(terminal, "Ferment V2: blocked", { full: false })
			const view = viewText(terminal)
			expect(view).toContain("Cache layer")
			expect(view).toContain("/ferment-v2 resume")
			trace.step("blocked footer keeps the objective and resume hint")
			terminal.submit("/ferment-v2 clear")
			await waitForText(terminal, "Ferment V2 cleared.", { full: false })
			expect(viewText(terminal)).not.toContain("Ferment V2:")
			trace.step("clearing the objective removes the unpinned V2 footer")
		},
	)
})
