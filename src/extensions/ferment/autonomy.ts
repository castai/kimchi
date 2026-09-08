/**
 * Session autonomy — one shared answer to "may a question interrupt a human?"
 *
 * A session is autonomous when no human may be interrupted mid-run, so every
 * question surface routes to the judge model: either the one-shot planner
 * flag is set, or the ferment runs under an automated continuation policy.
 * The ask-user audience router and the agent-communication contact router
 * both read from here so the rule lives in exactly one place.
 */

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent"
import type { FermentRuntime } from "./runtime.js"

/** True when the current PI session is the one-shot planner — no human is
 *  attached, so any question must route to the judge. */
function isOneShotSession(pi: ExtensionAPI): boolean {
	return pi.getFlag?.("ferment-oneshot") === true
}

/** True when no human should be interrupted mid-run: the one-shot flag is
 *  set, or the ferment runs under an automated continuation policy. Questions
 *  route to the judge in both cases. */
export function isAutonomousSession(
	pi: ExtensionAPI,
	runtime?: Pick<FermentRuntime, "getContinuationPolicy">,
): boolean {
	return isOneShotSession(pi) || runtime?.getContinuationPolicy() === "automated"
}

/** The judge a question can be routed to, when one exists: the session is
 *  autonomous AND a ferment is active to give the judge its goal/criteria
 *  context. Undefined means fall through to the next audience route. */
export function resolveAutonomousJudgeRoute(
	pi: ExtensionAPI,
	runtime?: Pick<FermentRuntime, "getContinuationPolicy" | "getActiveId">,
): { fermentId: string } | undefined {
	if (!runtime || !isAutonomousSession(pi, runtime)) return undefined
	const fermentId = runtime.getActiveId()
	return fermentId ? { fermentId } : undefined
}
