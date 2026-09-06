import { WORKSPACE_RESOURCE_FIELDS, type WorkspaceResourcesConfig } from "./types.js"
import { WORKSPACE_FILE_NAME } from "./workspace-file.js"

/**
 * Thrown when a resource value in `kimchi_workspace.yaml` is not a valid,
 * positive Kubernetes quantity. The message names the field and the offending
 * value; surfaced as a refusal before any network call.
 */
export class WorkspaceResourcesError extends Error {
	constructor(message: string) {
		super(message)
		this.name = "WorkspaceResourcesError"
	}
}

/**
 * Kubernetes quantity: numeric part (optionally decimal), then EITHER an
 * exponent OR a decimal-SI (n,u,m,k,M,G,T,P,E) / binary-SI (Ki…Ei) suffix —
 * never both (`1e3m` passes client validation only to 400 server-side, so it
 * is rejected here). Group 1 captures the mantissa for the positivity check.
 */
const QUANTITY_RE = /^([0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:(?:[eE][+-]?[0-9]+)|(?:n|u|m|k|M|G|T|P|E|Ki|Mi|Gi|Ti|Pi|Ei))?$/

/**
 * Validate and normalize resource requests from `kimchi_workspace.yaml`.
 *
 * The client owns syntax only — values pass through to the server verbatim
 * as quantity strings; no millicore/byte conversion here. Normalization is
 * outer-whitespace trimming only — internal whitespace makes the value
 * invalid (a typo like "2 0Gi" must never silently become "20Gi").
 * Positivity is enforced (zero and unparseable values rejected); a field
 * left unset is omitted (inherits org policy). Returns undefined when no
 * resources are set at all.
 */
export function resolveWorkspaceResources(
	config: WorkspaceResourcesConfig | undefined,
): WorkspaceResourcesConfig | undefined {
	if (!config) return undefined
	const out: WorkspaceResourcesConfig = {}
	for (const field of WORKSPACE_RESOURCE_FIELDS) {
		const raw = config[field]
		if (raw === undefined) continue
		const normalized = raw.trim()
		const match = QUANTITY_RE.exec(normalized)
		if (!match) {
			throw new WorkspaceResourcesError(
				`Invalid ${field} value "${raw}" in ${WORKSPACE_FILE_NAME} — expected a Kubernetes quantity (e.g. "500m", "1Gi", "20Gi").`,
			)
		}
		if (Number.parseFloat(match[1]) <= 0) {
			throw new WorkspaceResourcesError(
				`Invalid ${field} value "${raw}" in ${WORKSPACE_FILE_NAME} — must be positive; remove the field to inherit the org default.`,
			)
		}
		out[field] = normalized
	}
	return Object.keys(out).length > 0 ? out : undefined
}
