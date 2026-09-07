import { describe, expect, it } from "vitest"
import { createModel } from "../__mocks__/model-registry.js"
import { DEFAULT_CLASSIFIER_CANDIDATE_REFS, resolveClassifierCandidates } from "./classifier-models.js"

const primary = createModel("deepseek-v4-flash-0731")
const fallback = createModel("minimax-m3")

describe("resolveClassifierCandidates", () => {
	it("pins the ordered kimchi-dev ladder", () => {
		expect(DEFAULT_CLASSIFIER_CANDIDATE_REFS).toEqual(["kimchi-dev/deepseek-v4-flash-0731", "kimchi-dev/minimax-m3"])
	})

	it.each([
		{ models: [fallback, primary], candidates: [primary, fallback], missingRefs: [] },
		{ models: [fallback], candidates: [fallback], missingRefs: [DEFAULT_CLASSIFIER_CANDIDATE_REFS[0]] },
		{ models: [primary], candidates: [primary], missingRefs: [DEFAULT_CLASSIFIER_CANDIDATE_REFS[1]] },
		{ models: [], candidates: [], missingRefs: [...DEFAULT_CLASSIFIER_CANDIDATE_REFS] },
		{
			models: [createModel(primary.id, "custom"), createModel(fallback.id, "custom")],
			candidates: [],
			missingRefs: [...DEFAULT_CLASSIFIER_CANDIDATE_REFS],
		},
	])("resolves ordered exact matches from $models", ({ models, candidates, missingRefs }) => {
		const registry = {
			find: (provider: string, id: string) => models.find((m) => m.provider === provider && m.id === id),
		}
		expect(resolveClassifierCandidates(registry)).toEqual({ candidates, missingRefs })
	})
})
