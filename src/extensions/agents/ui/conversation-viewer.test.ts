import { describe, expect, it } from "vitest"
import { formatToolArgs } from "./conversation-viewer.js"

describe("formatToolArgs", () => {
	it("shows the command for bash", () => {
		expect(formatToolArgs({ command: "cd some dir && cat file.txt" })).toBe("cd some dir && cat file.txt")
	})

	it("shows file_path for read/edit/write", () => {
		expect(formatToolArgs({ file_path: "/tmp/foo.ts" })).toBe("/tmp/foo.ts")
		expect(formatToolArgs({ path: "/tmp/foo.ts" })).toBe("/tmp/foo.ts")
	})

	it("shows url for web_fetch", () => {
		expect(formatToolArgs({ url: "https://example.com" })).toBe("https://example.com")
	})

	it("shows pattern for grep", () => {
		expect(formatToolArgs({ pattern: "foo\\d+", path: "src/" })).toBe("foo\\d+")
	})

	it("returns empty string for missing or empty args", () => {
		expect(formatToolArgs(undefined)).toBe("")
		expect(formatToolArgs(null)).toBe("")
		expect(formatToolArgs({})).toBe("")
		expect(formatToolArgs("string-not-object")).toBe("")
	})

	it("falls back to compact JSON for unknown arg shapes", () => {
		expect(formatToolArgs({ question: "continue?", answers: ["yes", "no"] })).toBe(
			'{"question":"continue?","answers":["yes","no"]}',
		)
	})
})
