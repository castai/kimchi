# Kimchi Cookbook and Recipes - User Guide

## Overview

The **Cookbook and Recipes** feature in Kimchi allows you to:

- **Export** your current AI tool configuration as a portable, versioned recipe
- **Share** configurations with your team via Git-based cookbooks
- **Install** pre-configured recipes from cookbooks
- **Manage** multiple AI tool setups with version control and upgrade capabilities
- **Fork** and customize existing recipes
- **Backup and restore** tool configurations

---

## Table of Contents

1. [Installing Kimchi from This Branch](#installing-kimchi-from-this-branch)
2. [Core Concepts](#core-concepts)
3. [Quick Start](#quick-start)
4. [Managing Cookbooks](#managing-cookbooks)
5. [Working with Recipes](#working-with-recipes)
6. [Recipe Installation](#recipe-installation)
7. [Recipe Export and Publishing](#recipe-export-and-publishing)
8. [Upgrading and Maintenance](#upgrading-and-maintenance)
9. [Backup and Restore](#backup-and-restore)
10. [Troubleshooting](#troubleshooting)

---

## Installing Kimchi from This Branch

Since this feature is on a development branch (`LLM-1212-add-kimchi-cookbook`), you need to build and install directly from source.

### Prerequisites

- Go 1.23 or later installed
- Git configured with your credentials
- Cast AI API key (get at https://kimchi.console.cast.ai)

> **Important:** Before running any `kimchi` command, set the following environment variable to prevent auto-update from replacing this pre-release binary with the latest official release (which does not include cookbook and recipe commands):
>
> ```bash
> export KIMCHI_NO_AUTO_UPDATE=1
> ```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/castai/kimchi.git
cd kimchi

# Checkout the cookbook branch
git checkout LLM-1212-add-kimchi-cookbook

# Build the binary
make build

# Install to /usr/local/bin (may require sudo)
make install

# Or install to ~/.local/bin
mkdir -p ~/.local/bin
cp bin/kimchi ~/.local/bin/
```

### Verify Installation

```bash
kimchi version
kimchi --help
```

---

## Core Concepts

### What is a Recipe?

A **recipe** is a YAML file that captures a complete AI tool configuration including:

- Provider settings and model configurations
- Agent configurations
- MCP (Model Context Protocol) server settings
- Custom skills and commands
- Theme files and plugins
- Referenced documentation files

Example recipe structure:
```yaml
name: my-awesome-config
version: 0.1.0
cookbook: my-team-cookbook
author: johndoe
description: Optimized OpenCode config for Go development
tags: ["go", "backend", "microservices"]
model: kimi-k2.5
use_case: coding
tools:
  opencode:
    providers:
      kimchi:
        name: Kimchi by Cast AI
        options:
          apiKey: kimchi:secret:KIMCHI_APIKEY
          baseURL: https://llm.cast.ai/openai/v1
    model: kimchi/kimi-k2.5
    # ... more configuration
```

### What is a Cookbook?

A **cookbook** is a Git repository that stores and versions recipes. It provides:

- Centralized recipe storage
- Version control via Git tags
- Team collaboration
- Recipe discovery and search

Cookbook structure:
```
my-cookbook/
├── .kimchi/
│   └── cookbook.yaml          # Cookbook metadata
├── recipes/
│   ├── python-debugger/
│   │   └── recipe.yaml        # A recipe
│   ├── go-backend/
│   │   └── recipe.yaml
│   └── react-frontend/
│       └── recipe.yaml
└── README.md
```

---

## Quick Start

### 1. List Available Recipes

See what recipes are available from registered cookbooks:

```bash
kimchi recipe list
```

Output example:
```
NAME              VERSION  COOKBOOK         AUTHOR      TOOLS      INSTALLED
----              -------  --------         ------      -----      ---------
python-debugger   1.2.0    castai-default   team        opencode   ✓
go-backend        0.5.0    castai-default   johndoe     opencode
react-frontend    2.1.0    castai-default   janedoe     opencode
```

### 2. Search for Recipes

Find recipes by name, description, or tags:

```bash
kimchi recipe search python
kimchi recipe search backend
```

### 3. View Recipe Details

```bash
kimchi recipe info python-debugger
kimchi recipe info castai-default/python-debugger
```

### 4. Install a Recipe

Install a recipe interactively:

```bash
kimchi recipe install python-debugger
```

Or preview without applying:

```bash
kimchi recipe install python-debugger --no-apply
```

### 5. Export Your Current Configuration

Save your current setup as a recipe:

```bash
kimchi recipe export
```

---

## Managing Cookbooks

### View Registered Cookbooks

```bash
kimchi cookbook list
```

### Add a Cookbook

Register a cookbook from a Git URL:

```bash
# Add from a GitHub repository
kimchi cookbook add https://github.com/organization/team-cookbook.git

# Add with custom name
kimchi cookbook add https://github.com/organization/team-cookbook.git --name team-recipes
```

Cookbooks are cloned to `~/.kimchi/cookbooks/<name>/`

### Create a New Cookbook

Scaffold and register a new cookbook:

```bash
# Create cookbook and push to remote (remote must already exist)
kimchi cookbook create https://github.com/myorg/my-cookbook.git
```

This creates:
- `recipes/` directory for storing recipes
- `.kimchi/cookbook.yaml` metadata file
- `README.md` with basic documentation

### Update Cookbooks

Pull the latest changes from all registered cookbooks:

```bash
# Update all cookbooks
kimchi cookbook update

# Update specific cookbook
kimchi cookbook update my-team-cookbook
```

### Default Cookbook

Kimchi automatically includes a default cookbook (currently pointing to `https://github.com/castai/kimchi-cookbook.git`). This is stored in `~/.kimchi/cookbooks/kimchi-cookbook/`.

To disable the default cookbook, set:
```bash
export KIMCHI_DEFAULT_COOKBOOK_URL=""
```

---

## Working with Recipes

### Recipe Naming Conventions

Recipes can be referenced in multiple ways:

```bash
# By name (searches all cookbooks)
kimchi recipe install python-debugger

# By name with version
kimchi recipe install python-debugger@1.2.0

# By cookbook/name
kimchi recipe install castai-default/python-debugger

# From local file
kimchi recipe install ./my-recipe.yaml
```

### Recipe Versions

Recipes follow semantic versioning (semver):
- `1.2.3` - Major.Minor.Patch
- Major: Breaking changes
- Minor: New features (backwards compatible)
- Patch: Bug fixes (backwards compatible)

### Recipe Metadata Fields

| Field | Description |
|-------|-------------|
| `name` | Unique recipe identifier |
| `version` | Semantic version (e.g., 0.1.0) |
| `cookbook` | Target cookbook for publishing |
| `author` | Recipe author |
| `description` | Short description |
| `tags` | Array of searchable tags |
| `model` | Primary model used |
| `use_case` | Intended use case |
| `created_at` | Creation timestamp |
| `updated_at` | Last update timestamp |

---

## Recipe Installation

### Interactive Installation

The recommended way to install recipes:

```bash
kimchi recipe install [source]
```

The wizard will guide you through:
1. **Recipe Source** - Enter a recipe name, version, or file path
2. **Recipe Preview** - Review what the recipe contains
3. **Select Assets** - Choose which assets to install
4. **Auth** *(if no API key stored)* - Authenticate with Cast AI
5. **Enter Secrets** *(if required)* - Enter third-party API keys
6. **Installing** - Apply the configuration

### Headless Installation

For automated/scripted installations:

```bash
# Using --yes flag (when available via upgrade)
kimchi recipe upgrade --yes
```

### Installation Sources

```bash
# From registered cookbook
kimchi recipe install python-debugger

# Specific version
kimchi recipe install python-debugger@1.2.0

# From file
kimchi recipe install ./custom-config.yaml

# From URL
kimchi recipe install https://example.com/recipes/custom.yaml
```

### List Installed Recipes

```bash
kimchi recipe list --installed
```

Output:
```
NAME              VERSION  COOKBOOK         TOOL       PINNED  INSTALLED AT
----              -------  --------         ----       ------  ------------
python-debugger   1.2.0    castai-default   opencode   ✓       2026-04-08 10:30
```

### Pinning Recipes

Prevent a recipe from being upgraded:

```bash
kimchi recipe pin python-debugger
```

Unpin to allow upgrades:

```bash
kimchi recipe unpin python-debugger
```

---

## Recipe Export and Publishing

### Export Your Configuration

Save your current AI tool setup as a recipe:

```bash
# Interactive export wizard
kimchi recipe export

# Specify output file
kimchi recipe export -o my-config.yaml

# Quick export with name
kimchi recipe export --name my-go-config --tag go --tag backend

# Dry run (preview only)
kimchi recipe export --dry-run
```

The export wizard guides you through:
1. **Select Tool** - Which tool configuration to export
2. **Config Scope** - What configuration to include
3. **Recipe Metadata** - Name, description, tags
4. **Export Use Case** - Intended use case for the recipe
5. **Include Assets** - Select additional files (skills, commands, etc.)
6. **Output** *(if not provided via `-o`)* - Choose save location

### Fork a Recipe

Create your own customizable copy of an existing recipe:

```bash
# Fork by name
kimchi recipe fork python-debugger

# Fork with new name
kimchi recipe fork python-debugger --name my-python-debugger

# Fork to specific file
kimchi recipe fork python-debugger -o ./my-fork.yaml
```

Forked recipes:
- Start at version `0.1.0`
- Have no cookbook assigned (resolved on first push)
- Include `forked_from` metadata tracking the origin

### Push a Recipe to Cookbook

Publish your recipe to a cookbook:

```bash
# Push with automatic version bump selection
kimchi recipe push my-recipe.yaml

# Bump patch version (1.2.3 → 1.2.4)
kimchi recipe push my-recipe.yaml --patch

# Bump minor version (1.2.3 → 1.3.0)
kimchi recipe push my-recipe.yaml --minor

# Bump major version (1.2.3 → 2.0.0)
kimchi recipe push my-recipe.yaml --major

# Metadata-only push (no version bump)
kimchi recipe push my-recipe.yaml --meta

# Push to specific cookbook
kimchi recipe push my-recipe.yaml --cookbook my-team-cookbook

# Preview push without applying
kimchi recipe push my-recipe.yaml --dry-run
```

**Version Bump Guidelines:**
- `--patch` - Bug fixes, documentation updates
- `--minor` - New features, backwards compatible
- `--major` - Breaking changes
- `--meta` - Only metadata changed (description, tags)

**Fork & PR Workflow:**
If you don't have write access to the cookbook, Kimchi will:
1. Authenticate via GitHub device flow
2. Fork the repository
3. Push your changes
4. Open a pull request

---

## Upgrading and Maintenance

### Upgrade All Recipes

Check for and install updates:

```bash
# Update cookbooks and upgrade recipes (interactive)
kimchi upgrade

# Dry run - see what would be updated
kimchi upgrade --dry-run
```

### Upgrade Specific Recipe

```bash
kimchi recipe upgrade python-debugger
```

### Update Cookbooks Only

Pull latest recipe definitions:

```bash
kimchi update
```

Or specifically:

```bash
kimchi cookbook update
```

### Automatic Updates

Kimchi automatically updates cookbooks once per day. To disable:

```bash
export KIMCHI_NO_AUTO_UPDATE=1
```

---

## Backup and Restore

Kimchi automatically creates backups before installing recipes.

### Automatic Backups

Before each recipe installation, Kimchi backs up the current tool configuration to:
- `~/.kimchi/backups/` (for all tools)

### Restore from Backup

Open the interactive restore wizard:

```bash
kimchi recipe restore
```

The wizard shows:
- **Baseline** - State before first recipe installation
- **Recipe backups** - Named backups from recipe installs
- Timestamps for each backup

Select a backup to restore, then confirm the operation.

### Backup Contents

Backups include:
- Full tool configuration files
- Custom skills and commands
- Theme files
- Plugin configurations
- Referenced documentation

---

## Troubleshooting

### Common Issues

#### Recipe Not Found

```bash
# Make sure cookbooks are up to date
kimchi cookbook update

# Search for the recipe
kimchi recipe search <partial-name>

# List all available recipes
kimchi recipe list
```

#### Push Fails - Permission Denied

Kimchi will automatically fork and create a PR if you don't have write access. Make sure:
- GitHub CLI (`gh`) is installed, OR
- You're prepared to authenticate via browser

#### Version Conflict

```bash
# Check current recipe version
kimchi recipe info <name>

# Use --patch, --minor, or --major to bump
kimchi recipe push <file> --patch
```

#### Installation Conflicts

Kimchi automatically backs up and removes existing config files before installing, so conflicts are handled transparently. If something goes wrong:
1. Use `--no-apply` to preview what would be installed
2. Use `kimchi recipe restore` to go back to the baseline or a previous state

### Getting Help

```bash
# General help
kimchi --help

# Command-specific help
kimchi recipe --help
kimchi recipe install --help
kimchi cookbook --help

# Enable debug output
kimchi --debug recipe install <name>
```

### Configuration Locations

| File/Directory | Purpose |
|----------------|---------|
| `~/.kimchi/cookbooks.json` | Registered cookbooks list |
| `~/.kimchi/cookbooks/` | Cloned cookbook repositories |
| `~/.kimchi/installed/` | Installed recipe metadata |
| `~/.kimchi/backups/` | Tool configuration backups |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `KIMCHI_API_KEY` | Cast AI API key |
| `KIMCHI_DEFAULT_COOKBOOK_URL` | URL for default cookbook (set to empty to disable) |
| `KIMCHI_NO_AUTO_UPDATE` | Set to `1` to disable automatic updates |

---

## Example Workflows

### Team Setup Workflow

```bash
# 1. Add team cookbook
kimchi cookbook add https://github.com/myorg/team-cookbook.git

# 2. List available team recipes
kimchi recipe list --cookbook team-cookbook

# 3. Install team standard config
kimchi recipe install team-standard

# 4. Pin important recipes
kimchi recipe pin team-standard
```

### Recipe Development Workflow

```bash
# 1. Start with a good base
kimchi recipe fork team-standard --name my-custom-config

# 2. Modify the YAML as needed
vim my-custom-config.yaml

# 3. Test locally
kimchi recipe install ./my-custom-config.yaml --no-apply

# 4. Create cookbook if needed
kimchi cookbook create https://github.com/myorg/personal-cookbook.git

# 5. Push initial version
kimchi recipe push ./my-custom-config.yaml --minor

# 6. Make updates and push patches
kimchi recipe push ./my-custom-config.yaml --patch
```

### Migration Workflow

```bash
# 1. Export current configuration
kimchi recipe export -o legacy-config.yaml

# 2. Test on new machine
# (On new machine)
kimchi recipe install ./legacy-config.yaml

# 3. Iterate and publish
kimchi recipe fork legacy-config.yaml --name production-config
kimchi recipe push production-config.yaml --cookbook team-cookbook --minor
```

---

## Summary of Commands

| Command | Description |
|---------|-------------|
| `kimchi cookbook list` | List registered cookbooks |
| `kimchi cookbook add <url>` | Add a cookbook |
| `kimchi cookbook create <url>` | Create and scaffold new cookbook |
| `kimchi cookbook update [name]` | Update cookbook(s) |
| `kimchi recipe list` | List available recipes |
| `kimchi recipe list --installed` | List installed recipes |
| `kimchi recipe search <query>` | Search recipes |
| `kimchi recipe info <name>` | Show recipe details |
| `kimchi recipe install <source>` | Install a recipe |
| `kimchi recipe export` | Export current config |
| `kimchi recipe fork <source>` | Fork a recipe |
| `kimchi recipe push <file>` | Push recipe to cookbook |
| `kimchi recipe upgrade [name]` | Upgrade recipe(s) |
| `kimchi recipe pin <name>` | Pin a recipe version |
| `kimchi recipe unpin <name>` | Unpin a recipe |
| `kimchi recipe restore` | Restore from backup |
| `kimchi update` | Update cookbooks and kimchi |
| `kimchi upgrade` | Update cookbooks and upgrade recipes |

---

## Feedback and Support

This feature is currently in development on the `LLM-1212-add-kimchi-cookbook` branch. For issues or feedback:

1. Check existing issues on the GitHub repository
2. Create a new issue with the `cookbook` label
3. Include the output of `kimchi version` and `kimchi --debug` if applicable

Happy cooking! 🥬
