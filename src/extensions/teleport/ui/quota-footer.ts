import type { QuotaUsage, ResourceUsage } from "../../../sandbox/cloud/types.js"
import { formatK8sBytesPair, formatMillicores } from "./format-bytes.js"

/**
 * Usage-vs-quota footer lines: the user scope on the first line, the org
 * scope on its own line below it — one line per scope keeps both fully
 * readable at common terminal widths instead of truncating the org tail.
 * Segments with missing fields are dropped; a scope with nothing to show
 * yields undefined, which callers replace with an empty row so the panel
 * always emits the same line count.
 *
 * Shared by the workspace picker (WorkspacesPanel) and the remote-sessions
 * panel so the two footers cannot drift apart.
 */
export function quotaLines(quota: QuotaUsage | undefined): [string | undefined, string | undefined] {
	const scope = (u: ResourceUsage): string | undefined => {
		const parts: string[] = []
		if (u.currentCpuMillicores !== undefined && u.maxCpuMillicores !== undefined) {
			parts.push(`${formatMillicores(u.currentCpuMillicores)}/${formatMillicores(u.maxCpuMillicores)} CPU`)
		}
		if (u.currentRamBytes !== undefined && u.maxRamBytes !== undefined) {
			parts.push(`${formatK8sBytesPair(u.currentRamBytes, u.maxRamBytes)} RAM`)
		}
		if (u.currentPvcSizeBytes !== undefined && u.maxPvcSizeBytes !== undefined) {
			parts.push(`${formatK8sBytesPair(u.currentPvcSizeBytes, u.maxPvcSizeBytes)} PVC`)
		}
		if (u.currentSandboxes !== undefined && u.maxSandboxes !== undefined) {
			parts.push(`${u.currentSandboxes}/${u.maxSandboxes} workspaces`)
		}
		return parts.length > 0 ? parts.join(" · ") : undefined
	}
	const user = quota?.userUsage ? scope(quota.userUsage) : undefined
	const org = quota?.orgUsage ? scope(quota.orgUsage) : undefined
	return [user ? `You: ${user}` : undefined, org ? `org: ${org}` : undefined]
}
