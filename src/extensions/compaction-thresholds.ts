/**
 * Shared compaction threshold constants.
 *
 * This module exists so multiple extensions (model-guard, ferment auto-compaction)
 * agree on the same reserve-tokens value used to decide when context is "full".
 *
 * The value MUST stay in sync with upstream `DEFAULT_COMPACTION_SETTINGS.reserveTokens`
 * (currently 16,384 tokens). If upstream changes, update this constant and the
 * corresponding check in model-guard.ts / ferment/auto-compaction.ts.
 */

/** Tokens reserved as headroom below the model's context window. */
export const COMPACTION_RESERVE_TOKENS = 16_384

/**
 * Error message fragments that upstream compaction paths treat as routine
 * "no-op" outcomes (session too small, already compacted, cancelled, another
 * compaction already in flight, nothing summarizable). Callers stay silent on
 * these and only warn on real failures. Single source of truth shared by the
 * model-guard and ferment compaction paths so they cannot drift apart.
 */
export const EXPECTED_COMPACTION_ERROR_MESSAGES = [
	"too small",
	"Already compacted",
	"Compaction cancelled",
	"Compaction already in progress",
	"no summarizable messages",
]

/** True when a compaction failure is a routine no-op rather than a real error. */
export function isExpectedCompactionError(error: Error): boolean {
	return EXPECTED_COMPACTION_ERROR_MESSAGES.some((message) => error.message.includes(message))
}
