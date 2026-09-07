/**
 * Parse a proto-JSON integer field. gRPC-gateway encodes int64 as a JSON
 * *string* ("1500") while int32 arrives as a number — accept both so callers
 * don't need to care about the wire width. Returns undefined when the field
 * is absent or not parseable as a finite number.
 */
export function parseInt64(raw: unknown): number | undefined {
	if (typeof raw === "number") return Number.isFinite(raw) ? raw : undefined
	if (typeof raw === "string" && raw.length > 0) {
		const n = Number(raw)
		if (Number.isFinite(n)) return n
	}
	return undefined
}
