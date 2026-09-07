import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent"

export interface FermentV2PlanExecution {
	readonly objective: string
	readonly title: string
	readonly planText: string
	readonly planPath?: string
}

export type FermentV2PlanExecutorResult = "started" | "kept-existing"
export type FermentV2PlanExecutor = (
	execution: FermentV2PlanExecution,
	ctx: ExtensionContext,
) => Promise<FermentV2PlanExecutorResult>

const FERMENT_V2_PLAN_EXECUTOR_LOOKUP_CHANNEL = "kimchi:ferment-v2:approved-plan-executor"

interface FermentV2PlanExecutorLookup {
	resolve(executor: FermentV2PlanExecutor): void
}

export function registerFermentV2PlanExecutor(pi: ExtensionAPI, executor: FermentV2PlanExecutor): () => void {
	return pi.events.on(FERMENT_V2_PLAN_EXECUTOR_LOOKUP_CHANNEL, (data) => {
		if (isFermentV2PlanExecutorLookup(data)) data.resolve(executor)
	})
}

export function getFermentV2PlanExecutor(pi: ExtensionAPI): FermentV2PlanExecutor | undefined {
	let executor: FermentV2PlanExecutor | undefined
	const lookup: FermentV2PlanExecutorLookup = {
		resolve(candidate) {
			executor ??= candidate
		},
	}
	pi.events.emit(FERMENT_V2_PLAN_EXECUTOR_LOOKUP_CHANNEL, lookup)
	return executor
}

export function buildApprovedPlanObjective(planPath: string | undefined, planText: string): string {
	const reference = planPath ? `\n\nSaved plan copy (reference only): ${JSON.stringify(planPath)}` : ""
	return (
		"Implement the approved plan below, complete its requirements, and verify the result. " +
		"This approved Markdown is authoritative even if the saved copy changes or is missing." +
		`${reference}\n\n<approved_plan>\n${planText}\n</approved_plan>`
	)
}

function isFermentV2PlanExecutorLookup(value: unknown): value is FermentV2PlanExecutorLookup {
	return typeof value === "object" && value !== null && "resolve" in value && typeof value.resolve === "function"
}
