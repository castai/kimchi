package recipe

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/tools"
)

// HeadlessInstallOptions controls non-interactive recipe installation.
type HeadlessInstallOptions struct {
	// Source is the recipe path, name, cookbook/name, or name@version.
	Source string
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

	filesToCapture, err := PredictAssetPaths(r)
	if err != nil {
		return fmt.Errorf("predict asset paths: %w", err)
	}
	baselineBacked, err := EnsureBaseline(tools.ToolOpenCode, filesToCapture)
	if err != nil {
		return fmt.Errorf("backup baseline: %w", err)
	}
	if err := RemoveAssetFiles(baselineBacked); err != nil {
		return fmt.Errorf("clean baseline assets: %w", err)
	}
	snapshotBacked, err := SnapshotCurrentlyInstalled(tools.ToolOpenCode)
	if err != nil {
		return fmt.Errorf("backup current recipes: %w", err)
	}
	if err := RemoveAssetFiles(snapshotBacked); err != nil {
		return fmt.Errorf("clean installed assets: %w", err)
	}

	written, err := InstallOpenCode(r, secretValues, nil)
	if err != nil {
		return err
	}

	// Save manifest (exclude opencode.json — it's a merge target, not verbatim).
	var assetFiles []string
	for _, p := range written {
		if filepath.Base(p) != "opencode.json" {
			assetFiles = append(assetFiles, p)
		}
	}
	_ = SaveManifest(&RecipeManifest{
		RecipeName:  r.Name,
		Tool:        tools.ToolOpenCode,
		InstalledAt: time.Now(),
		AssetFiles:  assetFiles,
	})
	_ = RecordInstall(r.Name, r.Version, r.Cookbook, tools.ToolOpenCode)
	return nil
}
