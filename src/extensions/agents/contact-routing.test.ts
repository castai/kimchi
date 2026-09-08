import { describe, expect, it } from "vitest"
import { resolveUserContact } from "./contact-routing.js"

describe("resolveUserContact", () => {
	it("prefers the judge over the questionnaire in autonomous sessions even with a UI attached", () => {
		expect(resolveUserContact({ hasUI: true, judgeRoute: { fermentId: "f-1" } })).toEqual({
			reachable: true,
			route: "ferment_judge",
			ferment_id: "f-1",
		})
	})

	it("falls back to the questionnaire when no judge is available and a UI is attached", () => {
		expect(resolveUserContact({ hasUI: true })).toEqual({ reachable: true, route: "questionnaire" })
	})

	it("ends at the unavailable terminal when no audience matches", () => {
		const contact = resolveUserContact({ hasUI: false })
		expect(contact).toMatchObject({ reachable: false, route: "unavailable" })
	})
})
