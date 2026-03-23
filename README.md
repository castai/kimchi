# Kimchi

**Configure your AI coding tools to use Cast AI's open-source models in seconds.**

[![Release](https://img.shields.io/github/v/release/castai/kimchi?include_prereleases)](https://github.com/castai/kimchi/releases)
[![License](https://img.shields.io/badge/license-Cast%20AI-blue)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/castai/kimchi)](https://goreportcard.com/report/github.com/castai/kimchi)

---

## What is Kimchi?

Kimchi is a CLI tool that configures your favorite AI coding assistants to use open-source models hosted by Cast AI:

| Model            | Best For                                                                    | Context | Output |
|------------------|-----------------------------------------------------------------------------|---------|--------|
| **glm-5-fp8**    | Reasoning, planning, analysis                                               | 202.8K tokens | 32K tokens |
| **minimax-m2.5** | Code generation, debugging                                                  | 196.6K tokens | 32K tokens |
| **kimi-k2.5**    | Reasoning, planning, analysis, code generation, debugging, image processing | 202.8K tokens | 32K tokens |

No API keys from Anthropic or OpenAI needed — just your Cast AI API key.

---

## Quick Start

### One-Line Install

```bash
curl -fsSL https://github.com/castai/kimchi/releases/latest/download/install.sh | bash
```

This downloads and installs Kimchi, then launches the setup wizard automatically.

### Manual Install

Download the latest release for your platform:

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS | Intel | `kimchi_darwin_amd64.tar.gz` |
| macOS | Apple Silicon | `kimchi_darwin_arm64.tar.gz` |
| Linux | x86_64 | `kimchi_linux_amd64.tar.gz` |
| Linux | ARM64 | `kimchi_linux_arm64.tar.gz` |

```bash
# Download and extract
curl -fsSL https://github.com/castai/kimchi/releases/latest/download/kimchi_linux_amd64.tar.gz | tar xzf -

# Make executable and move to PATH
chmod +x kimchi
sudo mv kimchi /usr/local/bin/
```

---

## Getting Started

### 1. Get Your API Key

1. Go to [console.cast.ai/user/api-access](https://console.cast.ai/user/api-access)
2. Create or copy your API key

### 2. Run Kimchi

```bash
kimchi
```

The interactive wizard will guide you through:

1. **Auth** — Enter and validate your Cast AI API key
2. **Detect Tools** — Automatically finds installed AI tools
3. **Select Tools** — Choose which tools to configure
4. **Scope** — Global (all projects) or project-specific
5. **GSD Setup** — Optional: Install Goal-Driven Development agents
6. **Configure** — Writes configuration files
7. **Done** — Ready to code!

---

## Supported Tools

| Tool | Description | Config File |
|------|-------------|-------------|
| [OpenCode](https://opencode.ai) | Agentic coding CLI | `~/.config/opencode/opencode.json` |
| [Claude Code](https://claude.ai/code) | Anthropic's coding agent | `~/.claude/settings.json` |
| [Codex](https://github.com/openai/codex) | OpenAI's coding CLI | `~/.codex/.env` |
| [Zed](https://zed.dev) | High-performance editor | `~/.zed/settings.json` |
| [Cline](https://cline.bot) | VS Code extension | `~/.cline/data/globalState.json` |
| Generic | Environment variables | Prints to stdout |

---

## Usage

```bash
# Launch interactive setup
kimchi

# Show version
kimchi version

# Enable debug output
kimchi --debug

# Generate shell completion
kimchi completion bash > /etc/bash_completion.d/kimchi
kimchi completion zsh > "${fpath[1]}/_kimchi"
kimchi completion fish > ~/.config/fish/completions/kimchi.fish
```

---

## How It Works

Kimchi configures each tool to use Cast AI's inference endpoint:

```
Your AI Tool ──► Kimchi Config ──► Cast AI Endpoint ──► Open-Source Models
                                        │
                                        ▼
                               https://llm.cast.ai
```

**Configuration Example (OpenCode):**

```json
{
  "provider": {
    "kimchi": {
      "name": "Kimchi by Cast AI",
      "options": {
        "baseURL": "https://llm.cast.ai/openai/v1",
        "apiKey": "your-api-key"
      },
      "models": {
        "glm-5-fp8": { "reasoning": true },
        "minimax-m2.5": { "reasoning": false }
      }
    }
  }
}
```

---

## FAQ

### Will this break my existing config?

No. Kimchi preserves your existing tool configurations and only adds its provider settings.

### Can I switch back?

Yes. Simply remove the `kimchi` provider from your tool's config file, or re-run the tool's original setup.

### Where is my API key stored?

- **Config file**: `~/.config/kimchi/config.json` (permissions: 600)
- **Environment variable**: `KIMCHI_API_KEY`

---

## Development

```bash
# Build
make build

# Test
make test

# Run locally
go run .

# Lint
golangci-lint run ./...
```

---

## License

Copyright © Cast AI. All rights reserved.
