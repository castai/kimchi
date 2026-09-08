/**
 * Subagent user-audience routing — who a child's user-addressed message
 * reaches right now.
 *
 * Precedence: the autonomous Ferment judge first, then the interactive
 * questionnaire, then no audience at all. Session liveness and root binding
 * are preconditions enforced by the caller (`src/extensions/agents/index.ts`).
 */

import type { AgentContact } from "./message-tool.js"

/** Live environment the resolution reads — nothing cached, everything
 *  resolvable at call time. */
export interface UserContactEnv {
	hasUI: boolean
	/** Present when the session is autonomous AND a judge can stand in for the
	 *  user (see `src/extensions/ferment/autonomy.ts`). */
	judgeRoute?: { fermentId: string }
}

/** Resolve who a subagent user-addressed message reaches right now. */
export function resolveUserContact(env: UserContactEnv): AgentContact {
	if (env.judgeRoute) return { reachable: true, route: "ferment_judge", ferment_id: env.judgeRoute.fermentId }
	if (env.hasUI) return { reachable: true, route: "questionnaire" }
	return {
		reachable: false,
		route: "unavailable",
		reason: "No live questionnaire or autonomous Ferment judge route is available.",
	}
}
