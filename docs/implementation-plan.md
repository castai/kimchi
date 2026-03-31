# Implementation Plan: Fix Project-Scoped Settings Path

## Goal
Fix project-scoped settings to use `.claude/settings.json` instead of `.settings.json`.

## Problem
When using `--scope project`, kimchi writes settings to `.settings.json` in the current directory instead of `.claude/settings.json`.

## Solution
Modify `ScopePaths` in `internal/config/scope.go` to return `<cwd>/.claude/settings.json` for project scope.

## Steps
1. ✅ Create branch from main
2. Edit `internal/config/scope.go`:
   - In `ScopePaths()` case for `ScopeProject`, return `.claude/settings.json` path
3. Update `internal/config/scope_test.go` to expect `.claude/settings.json`
4. Run tests
5. Push branch, create PR