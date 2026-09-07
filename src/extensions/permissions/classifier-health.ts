import type { Api, Model } from "@earendil-works/pi-ai"
import {
	type ClassifierDegradedPayload,
	type ClassifierUnavailablePayload,
	PERMISSION_EVENTS,
} from "./permissions-events.js"
import type { ClassifierResult } from "./types.js"

type ClassifierHealth =
	| {
			channel: typeof PERMISSION_EVENTS.CLASSIFIER_UNAVAILABLE
			payload: ClassifierUnavailablePayload
			message: string
	  }
	| {
			channel: typeof PERMISSION_EVENTS.CLASSIFIER_DEGRADED
			payload: ClassifierDegradedPayload
			message: string
	  }

/** Keep free-form model/provider diagnostics out of health events and notifications. */
export function classifierHealth(
	result: ClassifierResult,
	candidates: readonly Model<Api>[],
	missingRefs: string[],
	signal?: AbortSignal,
): ClassifierHealth | undefined {
	if (signal?.aborted || result.failureCode === "aborted") return undefined
	if (!result.ok) {
		return {
			channel: PERMISSION_EVENTS.CLASSIFIER_UNAVAILABLE,
			payload: {
				failureCode: result.failureCode ?? (candidates.length ? "provider_error" : "no_candidates"),
				missingRefs,
			},
			message:
				"Permissions classifier unavailable. Calls requiring classification need confirmation or are blocked without a UI.",
		}
	}
	if (result.usedModelId && (missingRefs.length > 0 || result.usedModelId !== candidates[0]?.id)) {
		return {
			channel: PERMISSION_EVENTS.CLASSIFIER_DEGRADED,
			payload: { usedModelId: result.usedModelId, missingRefs },
			message: "Permissions classifier is using a fallback model or has reduced model availability.",
		}
	}
	return undefined
}
