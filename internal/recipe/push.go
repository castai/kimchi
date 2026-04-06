package recipe

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/castai/kimchi/internal/config"
	"github.com/castai/kimchi/internal/cookbook"
)

// PushOptions controls the push behaviour.
type PushOptions struct {
	// File is the recipe yaml to push.
	File string
	// CookbookName overrides the cookbook to push to (otherwise taken from the recipe).
	CookbookName string
	// Version bump flags — at most one may be true.
	Patch bool
	Minor bool
	Major bool
	// Meta allows pushing without a version bump when only metadata changed.
	Meta bool
	// DryRun prints what would happen without writing or pushing.
	DryRun bool
}

// Push publishes a recipe to its target cookbook.
//
// If the user has write access to the cookbook's remote the recipe is committed
// and tagged directly. Otherwise a GitHub device-auth flow is triggered,
// the cookbook is forked, and a pull request is opened.
func Push(opts PushOptions, logFn func(string)) error {
	// ── 1. Read the recipe ──────────────────────────────────────────────────

	r, err := ReadFromFile(opts.File)
	if err != nil {
		return fmt.Errorf("read recipe: %w", err)
	}

	// ── 2. Resolve the target cookbook ─────────────────────────────────────

	cb, err := resolveCookbook(r, opts.CookbookName)
	if err != nil {
		return err
	}
	// If cookbook was resolved by auto-selection, write it back to the recipe file.
	if r.Cookbook != cb.Name {
		r.Cookbook = cb.Name
	}

	// ── 2a. Sync cookbook with remote ─────────────────────────────────────
	// Always reset to the remote state before doing any work. This handles
	// both the stale-clone case and the diverged-after-failed-push case.

	if syncErr := cookbook.SyncToRemote(cb.Path); syncErr != nil {
		logFn("warning: could not sync cookbook: " + syncErr.Error())
	}

	// ── 3. Check for version bump requirement ──────────────────────────────

	existingPath := cb.RecipePath(r.Name)
	var existing *Recipe
	if data, err := os.ReadFile(existingPath); err == nil {
		var ex Recipe
		if yaml.Unmarshal(data, &ex) == nil {
			existing = &ex
		}
	}

	// ── 3a. Compute the tag for the current version ────────────────────────
	// Do this before the "nothing changed" check so we can detect a previous
	// interrupted push (local tag exists but remote push failed).

	bumpedVersion := r.Version
	switch {
	case opts.Major:
		bumpedVersion, err = BumpMajor(r.Version)
	case opts.Minor:
		bumpedVersion, err = BumpMinor(r.Version)
	case opts.Patch:
		bumpedVersion, err = BumpPatch(r.Version)
	}
	if err != nil {
		return fmt.Errorf("bump version: %w", err)
	}
	tag := r.Name + "@" + bumpedVersion

	// If the local tag already exists a previous push was interrupted after
	// tagging but before a successful push — skip the "nothing changed" guard.
	previouslyInterrupted := cookbook.TagExists(cb.Path, tag)

	if existing != nil && !previouslyInterrupted {
		bodyChanged := recipeBodyChanged(existing, r)
		if bodyChanged && !opts.Patch && !opts.Minor && !opts.Major {
			return fmt.Errorf(
				"recipe body has changed but no version bump flag was provided\n"+
					"  current version: %s\n"+
					"  use --patch, --minor, or --major to bump the version",
				r.Version,
			)
		}
		if !bodyChanged && !opts.Meta && !opts.Patch && !opts.Minor && !opts.Major {
			return fmt.Errorf("nothing to push — recipe is unchanged (use --meta to push metadata-only changes)")
		}
	}

	// ── 4. Apply the pre-computed version bump ─────────────────────────────

	r.Version = bumpedVersion
	r.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	logFn(fmt.Sprintf("pushing %s as %s", r.Name, tag))

	if opts.DryRun {
		logFn("dry-run: would write recipe to " + existingPath)
		logFn("dry-run: would commit, tag " + tag + ", and push")
		return nil
	}

	// ── 5. Write bumped version back to the source file ────────────────────

	if err := WriteYAML(opts.File, r); err != nil {
		return fmt.Errorf("write recipe file: %w", err)
	}

	// ── 6. Copy recipe into the cookbook ──────────────────────────────────

	if err := os.MkdirAll(filepath.Dir(existingPath), 0755); err != nil {
		return fmt.Errorf("create recipe dir: %w", err)
	}
	data, err := os.ReadFile(opts.File)
	if err != nil {
		return err
	}
	if err := os.WriteFile(existingPath, data, 0644); err != nil {
		return fmt.Errorf("write recipe to cookbook: %w", err)
	}

	// ── 7. Commit ──────────────────────────────────────────────────────────

	relPath, _ := filepath.Rel(cb.Path, existingPath)
	if err := cookbook.AddFiles(cb.Path, []string{relPath}); err != nil {
		return err
	}
	commitMsg := fmt.Sprintf("Add %s", tag)
	if existing != nil {
		commitMsg = fmt.Sprintf("Update %s", tag)
	}
	if err := cookbook.Commit(cb.Path, commitMsg); err != nil {
		return err
	}
	if !cookbook.TagExists(cb.Path, tag) {
		if err := cookbook.CreateTag(cb.Path, tag); err != nil {
			return err
		}
	}

	// ── 8. Push (direct or via GitHub PR) ─────────────────────────────────

	logFn("pushing to " + cb.URL + "…")
	hasAccess, err := cookbook.Push(cb.Path)
	if err != nil {
		return err
	}
	if hasAccess {
		// Push the tag only after confirming direct write access — avoids
		// leaking tags to the upstream repo when branch protection forces a PR.
		if err := cookbook.PushTag(cb.Path, tag); err != nil {
			return err
		}
		logFn("✓ pushed " + tag)
		return nil
	}

	// No write access → GitHub fork + PR flow
	logFn("no write access — starting GitHub fork flow")
	return pushViaGitHubPR(r, cb, tag, relPath, opts.File, logFn)
}

// pushViaGitHubPR forks the cookbook on GitHub, pushes a branch, and opens a PR.
func pushViaGitHubPR(r *Recipe, cb *cookbook.Cookbook, tag, relPath, sourceFile string, logFn func(string)) error {
	// Load stored GitHub token, or run device flow now.
	token, err := config.GetGitHubToken()
	if err != nil || token == "" {
		logFn("No GitHub token found — starting device authorisation flow")
		token, err = runGitHubDeviceFlow(logFn)
		if err != nil {
			return fmt.Errorf("GitHub auth: %w", err)
		}
		if saveErr := config.SetGitHubToken(token); saveErr != nil {
			logFn("warning: could not save GitHub token: " + saveErr.Error())
		}
	}

	username, err := cookbook.GetUsername(token)
	if err != nil {
		return fmt.Errorf("get GitHub username: %w", err)
	}

	// Force the author to match the authenticated GitHub username.
	if r.Author != username {
		r.Author = username
		if err := WriteYAML(sourceFile, r); err != nil {
			return fmt.Errorf("update author in recipe file: %w", err)
		}
		logFn(fmt.Sprintf("author set to %s", username))
	}

	owner, repo, err := cookbook.ParseGitHubURL(cb.URL)
	if err != nil {
		return fmt.Errorf("parse cookbook URL: %w", err)
	}

	// Fork the cookbook if needed
	logFn(fmt.Sprintf("forking %s/%s…", owner, repo))
	forkURL, err := cookbook.ForkRepo(token, owner, repo)
	if err != nil {
		return fmt.Errorf("fork cookbook: %w", err)
	}

	// Clone fork into a temp dir
	tmp, err := os.MkdirTemp("", "kimchi-push-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	cloneURL := cookbook.TokenCloneURL(token, forkURL)
	logFn("cloning fork…")
	if err := cookbook.Clone(cloneURL, tmp); err != nil {
		return fmt.Errorf("clone fork: %w", err)
	}

	// Sync the fork clone to the upstream so the branch has no conflicts.
	upstreamURL := cookbook.TokenCloneURL(token, cb.URL)
	logFn("syncing fork with upstream…")
	if err := cookbook.AddRemote(tmp, "upstream", upstreamURL); err != nil {
		return fmt.Errorf("add upstream remote: %w", err)
	}
	if err := cookbook.SyncForkToUpstream(tmp); err != nil {
		return fmt.Errorf("sync fork to upstream: %w", err)
	}

	branch := "add/" + tag
	if err := cookbook.CreateBranch(tmp, branch); err != nil {
		return fmt.Errorf("create branch: %w", err)
	}

	// Copy recipe into temp clone
	destPath := filepath.Join(tmp, relPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	data, err := os.ReadFile(cb.RecipePath(r.Name))
	if err != nil {
		return err
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return err
	}

	if err := cookbook.AddFiles(tmp, []string{relPath}); err != nil {
		return err
	}
	if err := cookbook.Commit(tmp, fmt.Sprintf("Add recipe %s", tag)); err != nil {
		return err
	}

	logFn("pushing branch to fork…")
	hasAccess, err := cookbook.PushBranch(tmp, branch)
	if err != nil {
		return err
	}
	if !hasAccess {
		return fmt.Errorf("could not push to fork — check GitHub token permissions")
	}

	// Open PR (or report if one already exists)
	head := username + ":" + branch
	prNum, prURL, _ := cookbook.FindOpenPR(token, owner, repo, head)
	if prNum > 0 {
		logFn(fmt.Sprintf("updated existing PR: %s", prURL))
		return nil
	}

	prURL, err = cookbook.CreatePR(
		token, owner, repo,
		head, "main",
		fmt.Sprintf("Add recipe %s", tag),
		fmt.Sprintf("Adding recipe `%s` version `%s`.", r.Name, r.Version),
	)
	if err != nil {
		return fmt.Errorf("create PR: %w", err)
	}
	logFn(fmt.Sprintf("✓ PR opened: %s", prURL))
	return nil
}

// resolveCookbook finds the target cookbook for a push.
// If only one cookbook is registered it is used automatically.
// If multiple exist and neither the recipe nor the --cookbook flag resolves one,
// it returns an error asking the user to set one.
func resolveCookbook(r *Recipe, flagCookbook string) (*cookbook.Cookbook, error) {
	cookbooks, err := cookbook.Load()
	if err != nil {
		return nil, fmt.Errorf("load cookbooks: %w", err)
	}
	if len(cookbooks) == 0 {
		return nil, fmt.Errorf("no cookbooks registered — use `kimchi cookbook add <url>` first")
	}

	target := flagCookbook
	if target == "" {
		target = r.Cookbook
	}

	if target != "" {
		for i, cb := range cookbooks {
			if cb.Name == target {
				return &cookbooks[i], nil
			}
		}
		return nil, fmt.Errorf("cookbook %q not found in registered cookbooks", target)
	}

	if len(cookbooks) == 1 {
		return &cookbooks[0], nil
	}

	// Multiple cookbooks and no explicit target — fall back to the default cookbook.
	for i, cb := range cookbooks {
		if cookbook.IsDefault(cb.Name) {
			return &cookbooks[i], nil
		}
	}

	names := make([]string, len(cookbooks))
	for i, cb := range cookbooks {
		names[i] = cb.Name
	}
	return nil, fmt.Errorf(
		"multiple cookbooks registered — specify one with --cookbook\n  available: %s",
		joinStrings(names, ", "),
	)
}

// recipeBodyChanged compares the tools section of two recipes.
func recipeBodyChanged(a, b *Recipe) bool {
	aBytes, _ := yaml.Marshal(a.Tools)
	bBytes, _ := yaml.Marshal(b.Tools)
	return !bytes.Equal(aBytes, bBytes)
}

// runGitHubDeviceFlow starts the GitHub device auth flow, prints the code and URL
// via logFn, polls until the user authorises, and returns the access token.
func runGitHubDeviceFlow(logFn func(string)) (string, error) {
	dcr, err := cookbook.RequestDeviceCode()
	if err != nil {
		return "", err
	}
	logFn(fmt.Sprintf("Open %s and enter code: %s", dcr.VerificationURL, dcr.UserCode))

	deadline := time.Now().Add(time.Duration(dcr.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(dcr.Interval) * time.Second)
		token, err := cookbook.PollForToken(dcr.DeviceCode)
		if err != nil {
			return "", err
		}
		if token != "" {
			return token, nil
		}
	}
	return "", fmt.Errorf("device auth timed out")
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
