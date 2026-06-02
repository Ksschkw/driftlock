# Driftlock

**Driftlock** is a commit‑time gatekeeper that ensures your documentation never
falls behind your code. It watches every `git commit`, detects when a
structural change to your code (function signatures, class methods, exported
types) is not reflected in your Markdown docs, and either blocks the commit
or **automatically rewrites the documentation** to match. No more “I’ll update
the docs later” – Driftlock makes stale documentation a build failure.

Optionally, Driftlock can log an immutable audit trail to a Solana devnet
contract, giving you cryptographic proof that documentation matched code at the
moment of every commit.

---

## Table of Contents

1. [How It Works](#how-it-works)  
2. [What Counts as a Structural Change](#what-counts-as-a-structural-change)  
3. [Installation](#installation)  
    - [Pre‑built binary](#pre-built-binary)  
    - [Shell installer](#shell-installer)  
    - [Via Go](#via-go)  
4. [Quick Start](#quick-start)  
5. [Configuration](#configuration)  
    - [`doc_mapping`](#doc_mapping)  
    - [`llm`](#llm)  
    - [`behavior`](#behavior)  
    - [`audit`](#audit)  
6. [LLM Providers](#llm-providers)  
7. [Commands](#commands)  
8. [Behavior & Workflow](#behavior--workflow)  
9. [Audit Trail (Solana)](#audit-trail-solana)  
10. [Development](#development)  
11. [License](#license)

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
   a. Send (diff + current doc) to an LLM
   b. LLM returns TRUE (docs OK) or FALSE (outdated)
   c. If FALSE → LLM rewrites the affected doc sections
   d. Print result, optionally block commit (exit 1)
5. If no structural changes: print a green status message, exit 0
6. (Optional) Log SHA-256 hash to local audit file or Solana
```

Driftlock is **not** a simple text‑diff checker. It only triggers when the
**API‑visible surface** of your code changes. It will stay completely silent
for cosmetic changes, comment edits, or internal refactors that don’t alter
public signatures.

---

## What Counts as a Structural Change

Driftlock uses language‑specific parsers to extract **function and method
signatures** (name, parameters, return types). Currently supported languages:

| Language | Extensions | Detected elements |
|----------|------------|-------------------|
| Python   | `.py`      | `def` functions, return type annotations |
| JavaScript / TypeScript | `.js`, `.ts`, `.jsx`, `.tsx` | `function` declarations, arrow functions assigned to `const`/`let` |
| Go       | `.go`      | `func` declarations, receiver methods, return types |
| Rust     | `.rs`      | `fn` declarations, parameters, return types |

If the **signature** changes (added/removed parameter, different return type,
renamed function), Driftlock will trigger. Adding a new function triggers it as
well, even if the docs never mentioned it before – Driftlock will ask the LLM
to create appropriate documentation.

**Non‑triggers:** modifying a function body, renaming a local variable, adding
a comment, changing a struct field (for now – struct/trait detection is coming).

If you want Driftlock to see the **full diff** (including bodies and comments)
when structural changes *are* present, enable `include_full_diff = true` in
your config. This gives the LLM more context to write richer documentation.

---

## Installation

### Pre‑built binary

Download the latest binary for your platform from the
[Releases page](https://github.com/Ksschkw/driftlock/releases). Place it
somewhere in your `PATH` (e.g., `/usr/local/bin`).

### Shell installer

```bash
curl -fsSL https://raw.githubusercontent.com/Ksschkw/driftlock/main/install.sh | sh
```

This script detects your OS and architecture, downloads the correct binary,
verifies its SHA‑256 checksum, and installs it to `/usr/local/bin`. Inspect the
script before running if you prefer.

### Via Go

```bash
go install github.com/Ksschkw/driftlock/cmd/driftlock@latest
```

Requires Go 1.22 or later.

---

## Quick Start

```bash
cd your-project
driftlock init                # creates .driftlock.toml, pre-commit hook, updates .gitignore
# edit .driftlock.toml to set your LLM provider and API key
git add . && git commit -m "your message"
# If your docs are out of sync, the commit is blocked and the docs are updated.
```

The `driftlock init` command also adds `.driftlock.toml` and `.driftlock/` to
`.gitignore` so secrets never leak.

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

- `sources` – a list of glob patterns matching source files. `**` matches all
  subdirectories.
- `docs` – a list of Markdown files or directories. If a directory (like
  `docs/`) is given, all `*.md` files inside are included.

You can have multiple `[[doc_mapping]]` sections.

### `llm`

```toml
[llm]
driver = "openai-compatible"       # or "ollama"
endpoint = "https://api.openrouter.ai/api/v1/chat/completions"
model = "deepseek/deepseek-chat"
api_key = "${DRIFTLOCK_API_KEY}"   # env var expansion
```

- `driver` – adapter to use: `openai-compatible` (OpenRouter, Groq, DeepSeek,
  Together, vLLM, etc.) or `ollama` (local Ollama).
- `endpoint` – the **full URL** of the chat completion endpoint, including
  `/v1/chat/completions` or `/api/generate`. Nothing is appended.
- `model` – the model name as the provider expects it.
- `api_key` – use `${ENV_VAR}` to reference an environment variable.
- `options` – a table of extra parameters passed directly to the API (e.g.,
  `temperature = 0.0`, `max_tokens = 4096`).
- `[llm.prompts]` – optional override of the built‑in check and fix prompts,
  with `{{ .Diff }}` and `{{ .Doc }}` placeholders.

### `behavior`

```toml
[behavior]
auto_fix = true
block_on_false = true
max_retries = 2
include_full_diff = false
```

- `auto_fix` – if `true`, Driftlock will rewrite the documentation when it
  detects drift.
- `block_on_false` – if `true`, the commit is aborted when drift is found.
- `max_retries` – number of retries with exponential backoff if the LLM request
  fails.
- `include_full_diff` – if `true`, the LLM receives the complete `git diff`
  (not just signature changes) when structural changes are present. This gives
  the model richer context for writing documentation, but uses more tokens.

### `audit`

```toml
[audit]
solana = false
rpc_endpoint = "https://api.devnet.solana.com"
keypair_path = "~/.config/solana/id.json"
program_id = ""
```

When `solana = true`, Driftlock submits each check’s hash to the Solana
blockchain using the built‑in Memo program (or your custom program). This
creates an immutable audit trail suitable for compliance.

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

To add a provider, just change the `endpoint` and `model` – no code changes
needed.

---

## Commands

| Command | Description |
|---------|-------------|
| `driftlock init` | Initialize the project (config, hook, .gitignore) |
| `driftlock hook-run` | Internal – called by the pre‑commit hook |
| `driftlock check` | Check for drift without modifying files; exit non‑zero if drift found |
| `driftlock fix` | Force regeneration of all mapped documentation |
| `driftlock log` | Show the last 20 audit log entries |

---

## Behavior & Workflow

When you run `git commit`:

1. If no mapped source files are staged, Driftlock exits silently.
2. If mapped sources have **no structural changes**, a green message is printed:
   `driftlock: No structural changes in mapped sources; documentation check skipped.`
3. If structural changes are found:
   - The LLM is called to check whether the current docs match the new code.
   - If the docs are **up‑to‑date** (LLM returns TRUE), a green message is
     printed and the commit proceeds.
   - If the docs are **outdated** (FALSE), a red message is printed. If
     `auto_fix = true`, the documentation is rewritten in place. The commit is
     blocked (exit code 1) so you can review and stage the updated doc.
   - If the LLM is unreachable after all retries, a yellow warning is shown
     and the commit proceeds – Driftlock never blocks a commit because of a
     network error.

Driftlock also supports a `DRIFTLOCK_DEBUG=1` environment variable that prints
the raw LLM response to stderr, useful for troubleshooting prompt issues.

---

## Audit Trail (Solana)

When enabled, Driftlock generates a SHA‑256 hash of the code diff and
documentation content, then submits it to the Solana blockchain using the
standard [Memo Program](https://spl.solana.com/memo). This hash is permanently
recorded and publicly verifiable. A local audit log is also kept in
`.driftlock/audit.jsonl`.

To use this feature:

1. Set `solana = true` and provide an RPC endpoint and funded keypair.
2. Ensure the account has enough SOL for transaction fees (devnet SOL is free
   via airdrops).

---

## Development

Requirements: Go 1.22+

```bash
git clone https://github.com/Ksschkw/driftlock.git
cd driftlock
go build -o driftlock ./cmd/driftlock
# To test locally, add the build directory to your PATH or run sudo cp driftlock /usr/local/bin/
```

Run `driftlock init` inside a test repository to set up the hook.

---

## License

MIT

---

*Outdated documentation is technical debt that compiles. Driftlock treats it
as a build failure.*
---