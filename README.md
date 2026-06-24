# Driftlock

**Driftlock** is a commit‑time gatekeeper that ensures your documentation never
falls behind your code. It watches every `git commit`, detects when a
structural change to your code (function signatures, class methods, exported
types) is not reflected in your Markdown docs, and either blocks the commit
or **automatically rewrites the documentation** to match. No more "I'll update
the docs later" – Driftlock makes stale documentation a build failure.

Optionally, Driftlock can log an immutable audit trail to a Solana devnet
contract, giving you cryptographic proof that documentation matched code at the
moment of every commit.

> Example: https://youtu.be/o-Ox7lnqHxs

---

## Table of Contents

1. [How It Works](#how-it-works)  
2. [What Counts as a Structural Change](#what-counts-as-a-structural-change)  
3. [Installation](#installation)  
4. [Quick Start](#quick-start)  
5. [Configuration](#configuration)  
6. [LLM Providers](#llm-providers)  
7. [Commands](#commands)  
8. [Behavior & Workflow](#behavior--workflow)  
9. [Audit Trail (Solana)](#audit-trail-solana)  
10. [Escaping the Hook](#escaping-the-hook)  
11. [Development](#development)  
12. [License](#license)

---

## How It Works

```
git commit
   │
   ▼
.git/hooks/pre-commit  →  driftlock hook-run
   │
   ▼
1. Capture staged diff (git diff --cached)
2. Parse old & new file versions to extract structural signatures
3. Map changed files → Markdown documents (from .driftlock.toml)
4. If structural changes exist:
   a. Extract only the relevant sections of the documentation (smart chunking)
   b. Send (diff + chunked doc) to an LLM
   c. LLM returns TRUE (docs OK) or FALSE (outdated)
   d. If FALSE → LLM rewrites the affected sections, which are then
      merged back into the full document. Commit is blocked.
5. If no structural changes: print a green status message, exit 0
6. (Optional) Log SHA-256 hash to local audit file or Solana
```

Driftlock is **not** a simple text‑diff checker. It only triggers when the
**API‑visible surface** of your code changes. It stays completely silent
for cosmetic changes, comment edits, or internal refactors that don't alter
public signatures.

---

## What Counts as a Structural Change

Driftlock uses a **universal parser** that recognizes structural elements
in all major programming and markup languages:

- **Functions/methods** in any language (Python, Go, JavaScript, Rust,
  Java, C, etc.)
- **Classes/interfaces/structs** definitions
- **Type declarations** (TypeScript, Go, Rust, etc.)
- **SQL** table/view/procedure definitions
- **YAML/JSON** keys at any nesting level
- **XML/HTML** tags
- **Markdown** headings
- **INI/TOML** sections
- **Shell** function definitions

The parser matches patterns like:
- `def function_name(params):` (Python)
- `function name(params) {` (JavaScript)
- `func name(params) returns` (Go)
- `fn name(params) -> type {` (Rust) 
- `public int method(params)` (Java/C#)
- `CREATE TABLE name` (SQL)
- `key: value` (YAML)
- `<tag>` (XML/HTML)

If the **signature** changes (added/removed parameter, different return type,
renamed function), Driftlock will trigger. Adding a new function triggers it as
well, even if the docs never mentioned it before – Driftlock will ask the LLM
to create appropriate documentation.

**Non‑triggers:** modifying a function body, renaming a local variable, adding
a comment, or changing whitespace/formatting.

If you want Driftlock to see the **full diff** (including bodies and comments)
when structural changes *are* present, enable `include_full_diff = true` in
your config. This gives the LLM more context to write richer documentation.

---

## Installation

### Unix (Linux & macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Ksschkw/driftlock/main/install.sh | sh
```

### Windows

Open PowerShell and run:

```powershell
Set-ExecutionPolicy Bypass -Scope Process -Force
iwr https://raw.githubusercontent.com/Ksschkw/driftlock/main/install.ps1 -UseBasicParsing | iex
```

### Via Go

```bash
go install github.com/Ksschkw/driftlock/cmd/driftlock@latest
```

### Manual download

Grab the correct binary for your platform from the
[Releases page](https://github.com/Ksschkw/driftlock/releases).

---

## Quick Start

```bash
cd your-project
driftlock init                # interactive guided setup
# edit .driftlock.toml to set your LLM provider and API key
git add . && git commit -m "your message"
# If your docs are out of sync, the commit is blocked and the docs are updated.
```

---

## Configuration

Driftlock looks for `.driftlock.toml` in your Git repository root. All fields
have sensible defaults; you only need to set your LLM provider.

### `doc_mapping`

```toml
[[doc_mapping]]
sources = ["cmd/**", "internal/**"]
docs = ["README.md"]
```

- `sources` – glob patterns matching source files. `**` matches all subdirectories.
- `docs` – Markdown files or directories. Directories are expanded to `*.md` files inside.

### `llm`

```toml
[llm]
driver = "openai-compatible"       # or "ollama"
endpoint = "https://api.openrouter.ai/api/v1/chat/completions"
model = "deepseek/deepseek-chat"
api_key = "${DRIFTLOCK_API_KEY}"   # env var expansion
```

- `driver` – adapter: `openai-compatible` (OpenRouter, Groq, DeepSeek, Together, vLLM) or `ollama`.
- `endpoint` – **full URL** of the chat completions endpoint. Nothing is appended.
- `model` – model name as the provider expects it.
- `api_key` – use `${ENV_VAR}` to reference an environment variable.
- `options` – extra parameters passed directly to the API (`temperature`, `max_tokens`, etc.).
- `[llm.prompts]` – optional override of the built‑in check and fix prompts.

### `behavior`

```toml
[behavior]
auto_fix = true
block_on_false = true
block_on_llm_error = false
max_retries = 2
include_full_diff = false
```

- `auto_fix` – if `true`, rewrites documentation when drift is detected.
- `block_on_false` – if `true`, aborts the commit when docs are outdated.
- `block_on_llm_error` – if `true`, aborts the commit when the LLM is unreachable.
- `max_retries` – number of retries with exponential backoff.
- `include_full_diff` – if `true`, sends the complete `git diff` to the LLM (uses more tokens).

### `audit`

```toml
[audit]
solana = false
rpc_endpoint = "https://api.devnet.solana.com"
keypair_path = "~/.config/solana/id.json"
program_id = ""
```

When `solana = true`, Driftlock submits each check’s hash to the Solana
blockchain using the built‑in Memo program.

---

## LLM Providers

Driftlock uses an adapter system. The `openai-compatible` driver works with
any service that exposes an OpenAI‑style chat completions endpoint, including:

- [OpenRouter](https://openrouter.ai) – one API key, hundreds of models.
- [Groq](https://console.groq.com) – ultra‑fast inference.
- [DeepSeek](https://platform.deepseek.com/api-docs) – cheap, powerful models.
- [Together AI](https://docs.together.ai/docs/openai-api-compatibility)
- Self‑hosted vLLM, Ollama with an OpenAI‑compatible wrapper, etc.

The `ollama` driver speaks the native Ollama API (default
`http://localhost:11434/api/generate`).

---

## Commands

| Command | Description |
|---------|-------------|
| `driftlock init` | Interactive guided setup (config, hook, .gitignore) |
| `driftlock hook-run [--no-fix]` | Called by the pre‑commit hook |
| `driftlock check` | Check for drift; never modifies files |
| `driftlock fix` | Force regeneration of all mapped documentation |
| `driftlock log` | Show the last 20 audit log entries |

---

## Behavior & Workflow

When you run `git commit`:

1. If no mapped source files are staged, Driftlock exits silently.
2. If mapped sources have **no structural changes**, a green message is printed.
3. If structural changes are found:
   - Only the relevant sections of the documentation are sent to the LLM
     (smart chunking).
   - If the docs are **up‑to‑date** (LLM returns TRUE), a green message is
     printed and the commit proceeds.
   - If the docs are **outdated** (FALSE), a red message is printed. If
     `auto_fix = true`, the LLM rewrites the affected sections, they are
     merged back into the full document, and the commit is blocked so you
     can review and stage the updated doc.
   - If the LLM is unreachable, a yellow warning is shown. By default the
     commit proceeds; set `block_on_llm_error = true` to change that.

Use `DRIFTLOCK_DEBUG=1` to see the raw LLM payloads and responses.

---

## Escaping the Hook

If you need to commit without triggering Driftlock (e.g., for bulk
infrastructure changes), set the environment variable:

```bash
DRIFTLOCK_SKIP=true git commit -m "your message"
```

This bypasses all checks and allows the commit to proceed immediately.

---

## Audit Trail (Solana)

When enabled, Driftlock generates a SHA‑256 hash of the code diff and
documentation content, then submits it to the Solana blockchain using the
standard [Memo Program](https://spl.solana.com/memo). This hash is permanently
recorded and publicly verifiable.

---

## Development

Requirements: Go 1.22+

```bash
git clone https://github.com/Ksschkw/driftlock.git
cd driftlock
go build -o driftlock ./cmd/driftlock
# To test locally, add the build directory to your PATH or run
# sudo cp driftlock /usr/local/bin/
```

---

## License

Driftlock is licensed under the **Business Source License 1.1 (BUSL‑1.1)**.
You may use, modify, and distribute the software freely for any non‑commercial
purpose, including personal use and internal use within an organisation.

**Hosting Driftlock as a service (SaaS) or building a directly competing**
**product requires a separate commercial license.**

The BUSL‑1.1 will automatically become MIT on **2099‑12‑31**.

If you need a commercial license for a prohibited use case, contact
`kookafor893@gmail.com`.

Full license text: [LICENSE](./LICENSE)

---
