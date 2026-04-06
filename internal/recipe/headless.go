package recipe

import (
	"fmt"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
)

// HeadlessInstallOptions controls non-interactive recipe installation.
type HeadlessInstallOptions struct {
	// Source is the recipe path, name, cookbook/name, or name@version.
	Source string
	// OverwriteConflicts controls whether existing files are overwritten.
	// When false, conflicting assets are skipped.
	OverwriteConflicts bool
}

// InstallHeadless installs a recipe without a TUI. It uses the stored Kimchi
// API key for provider secrets and skips recipes that require external secrets
// not yet available non-interactively. Returns an error with details when
// external secrets are required.
func InstallHeadless(opts HeadlessInstallOptions) error {
	r, err := ResolveSource(opts.Source)
	if err != nil {
		return fmt.Errorf("resolve recipe: %w", err)
	}

	// Build secret values: start with kimchi provider placeholders.
	secretValues := make(map[string]string)
	apiKey, _ := config.GetAPIKey()
	if apiKey != "" {
		all := make(map[string]struct{})
		CollectAllSecretPlaceholders(r, all)
		external := DetectExternalSecretPlaceholders(r)
		externalSet := make(map[string]struct{}, len(external))
		for _, p := range external {
			externalSet[p] = struct{}{}
		}
		for p := range all {
			if _, isExternal := externalSet[p]; !isExternal {
				secretValues[p] = apiKey
			}
		}
	}

	// Reject recipes that need external secrets — can't prompt headlessly.
	if external := DetectExternalSecretPlaceholders(r); len(external) > 0 {
		return fmt.Errorf(
			"recipe %q requires external secrets that cannot be filled non-interactively: %v\n"+
				"  use `kimchi recipe install %s` to fill them interactively",
			r.Name, external, opts.Source,
		)
	}

	// Build conflict decisions: all overwrite or all skip.
	conflicts, err := DetectConflicts(r)
	if err != nil {
		return fmt.Errorf("detect conflicts: %w", err)
	}
	decisions := make(AssetDecisions, len(conflicts))
	for _, c := range conflicts {
		decisions[c.Path] = opts.OverwriteConflicts
	}

	if err := InstallOpenCode(r, secretValues, decisions); err != nil {
		return err
	}
	_ = RecordInstall(r.Name, r.Version, r.Cookbook, tools.ToolOpenCode)
	return nil
}
