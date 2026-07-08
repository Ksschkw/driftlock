# Configuration reference

Driftlock is configured by a single file, `.driftlock.toml`, located at the **Git root** (the directory containing `.git`). Driftlock finds the root by walking up from your current directory.

Every field is optional. When the file is missing entirely, Driftlock uses built-in defaults. When a field is omitted, that field's default applies. `driftlock init` writes a complete file for you interactively — see [Getting started](./getting-started.md).

## Top-level structure

```toml
[[doc_mapping]]     # repeatable: maps source globs to docs
  sources = [...]
  docs = [...]

[llm]               # which model to call and how
  driver = "..."
  endpoint = "..."
  model = "..."
  api_key = "..."
  [llm.options]     # extra request-body params, merged verbatim
  [llm.prompts]     # optional check/fix prompt overrides

[behavior]          # how Driftlock acts on drift
[audit]             # optional Solana audit trail
```

---

## `[[doc_mapping]]`

A **repeatable** table (note the double brackets) that maps source files to the documentation that describes them. Each entry answers: "when these source files change, which docs must reflect it?"

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `sources` | list of glob strings | `["src/**"]` | Source file patterns to watch. |
| `docs` | list of strings | `["README.md", "docs/"]` | Markdown files, or directories. A **directory expands to the `*.md` files inside it**. |

### Glob syntax

| Token | Matches |
| --- | --- |
| `*` | Any sequence of characters **within a single path segment** (does not cross `/`). |
| `**` | Any number of path segments (crosses `/`). E.g. `internal/**`, `src/**/*.go`. |
| `?` | Exactly one character. |

### Examples

Watch one tree, document in the README:

```toml
[[doc_mapping]]
sources = ["src/**"]
docs = ["README.md"]
```

Multiple mappings — the API package is documented by a directory of Markdown, the CLI by a single page:

```toml
[[doc_mapping]]
sources = ["internal/api/**/*.go"]
docs = ["docs/api/"]          # every *.md inside docs/api/

[[doc_mapping]]
sources = ["cmd/**"]
docs = ["docs/cli.md"]
```

Scope narrowly to avoid over-triggering (see [Ignoring symbols](./ignoring.md)):

```toml
[[doc_mapping]]
sources = ["src/public/**", "pkg/**/*.go"]
docs = ["README.md", "docs/reference/"]
```

---

## `[llm]`

Selects and configures the model Driftlock calls to judge drift and to rewrite docs.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `driver` | string | `"ollama"` | Which adapter to use. See table below. |
| `endpoint` | string | `"http://localhost:11434"` | The **full chat-completions URL**. Nothing is appended to it. Supports `${ENV_VAR}` expansion. |
| `model` | string | `"codestral:22b"` | Model name as the provider expects it. |
| `api_key` | string | `""` | API key. Supports `${ENV_VAR}` expansion (e.g. `"${DRIFTLOCK_API_KEY}"`). Sent as `Authorization: Bearer <key>` for OpenAI-compatible drivers. |

### `driver` values

| Driver | Adapter used |
| --- | --- |
| `openai-compatible` | OpenAI-compatible adapter |
| `groq` | OpenAI-compatible adapter |
| `openrouter` | OpenAI-compatible adapter |
| `deepseek` | OpenAI-compatible adapter |
| `vllm` | OpenAI-compatible adapter |
| `ollama` | Native Ollama adapter |

All of `openai-compatible`, `groq`, `openrouter`, `deepseek`, and `vllm` route through the **same OpenAI-compatible adapter** — they differ only as labels for your own clarity. Use `ollama` for a native local Ollama server.

> **Important:** `endpoint` must be the *complete* chat-completions URL — for OpenRouter that is `https://openrouter.ai/api/v1/chat/completions`, not just the host. Driftlock POSTs to exactly the URL you give it. See [Providers](./providers.md) for the correct URL per provider.

### `[llm.options]`

A free-form table whose keys are **merged verbatim into the request body**. Use it to pass any provider parameter:

```toml
[llm.options]
temperature = 0.0
max_tokens = 4096
top_p = 1.0
```

Recommended: `temperature = 0.0` for deterministic, cache-friendly verdicts.

### `[llm.prompts]` — prompt overrides (optional)

Override the built-in prompts. Both fields are **Go `text/template`** strings and support `${ENV_VAR}` expansion.

| Field | Placeholders | Used for |
| --- | --- | --- |
| `check` | `{{ .Diff }}`, `{{ .Doc }}` | The drift verdict prompt. The model must answer `TRUE`/`FALSE` plus a one-sentence reason. |
| `fix` | `{{ .Diff }}`, `{{ .Doc }}` | The documentation-rewrite prompt. |

```toml
[llm.prompts]
check = '''
You are a strict API documentation auditor.
Structural code changes:
{{ .Diff }}

Current documentation section:
{{ .Doc }}

Answer TRUE if the documentation already reflects the changes, otherwise FALSE,
followed by a one-sentence reason.
'''
fix = '''
Rewrite the documentation section below so it accurately reflects the code changes.
Return only the corrected Markdown, preserving all headings exactly.

Code changes:
{{ .Diff }}

Documentation to fix:
{{ .Doc }}
'''
```

- `{{ .Diff }}` is the structural signature diff by default, or the full git diff if `include_full_diff = true`.
- `{{ .Doc }}` is the smart-chunked doc section(s) mentioning the changed symbols.
- If you omit `[llm.prompts]`, Driftlock's built-in prompts are used. The `fix` prompt must return corrected Markdown that preserves headings, because rewritten chunks are merged back by **exact heading match**.

---

## `[behavior]`

Controls how Driftlock acts once drift is detected.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `auto_fix` | bool | `true` | When drift is found, rewrite the affected doc sections via the LLM. The commit is still blocked so you can review. |
| `block_on_false` | bool | `true` | Block the commit when a check verdict is `FALSE` (docs outdated). |
| `block_on_llm_error` | bool | `false` | When the LLM is unreachable, block the commit instead of allowing it with a warning. |
| `max_retries` | int | `2` | Number of LLM retries on transient failure, using exponential backoff. |
| `include_full_diff` | bool | `false` | Send the **full git diff** to the LLM instead of only the structural signature changes. More context, but more tokens. |
| `cache` | bool | `true` | Enable the content-addressed verdict cache (`.driftlock/cache.json`). Identical `(model, diff, doc)` checks are never re-sent to the LLM. **On by default.** |

### Notes

- **`cache` defaults to `true`.** Omitting it, or setting `cache = true`, keeps caching on. Set `cache = false` to disable. See [Caching](./caching.md).
- **`block_on_llm_error`** is a policy choice: `false` (default) favors developer flow (commit proceeds with a warning if the LLM is down); `true` favors strictness (never let a commit through unverified). CI often wants `true`.
- **`include_full_diff`** trades cost for context. Leave it off unless the LLM struggles to judge drift from the structural diff alone. See [Providers → economics](./providers.md).

```toml
[behavior]
auto_fix = true
block_on_false = true
block_on_llm_error = false
max_retries = 2
include_full_diff = false
cache = true
```

---

## `[audit]`

An optional, tamper-evident audit trail. Driftlock always appends a SHA-256 of `(diff + doc)` to a local log at `.driftlock/audit.jsonl`; the `[audit]` section additionally anchors those hashes on **Solana**.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `solana` | bool | `false` | Enable writing the audit hash to Solana. |
| `rpc_endpoint` | string | `""` | Solana RPC endpoint (e.g. `https://api.devnet.solana.com`). |
| `keypair_path` | string | `""` | Path to the Solana keypair that signs the audit transactions (e.g. `~/.config/solana/id.json`). |
| `program_id` | string | `""` | Program to write to. **Empty uses the default Solana Memo program.** |

```toml
[audit]
solana = true
rpc_endpoint = "https://api.devnet.solana.com"
keypair_path = "~/.config/solana/id.json"
program_id = ""     # default Memo program
```

View the last 20 local audit entries any time with:

```bash
driftlock log
```

---

## Environment variables

| Variable | Effect |
| --- | --- |
| `DRIFTLOCK_API_KEY` | Conventional variable referenced from config as `api_key = "${DRIFTLOCK_API_KEY}"`. Also the variable the [GitHub Action](./ci-cd.md) sets from your repo secret. |
| `DRIFTLOCK_DEBUG=1` | Prints the raw LLM request/response payloads **and token usage** to stderr. Invaluable for debugging prompts and cost. |
| `DRIFTLOCK_SKIP=true` | Bypasses the pre-commit hook for a single commit, e.g. `DRIFTLOCK_SKIP=true git commit -m "..."`. |

Any `${ENV_VAR}` in `api_key`, `endpoint`, or the `[llm.prompts]` strings is expanded at load time.

---

## A complete example

```toml
[[doc_mapping]]
  sources = ["cmd/**", "internal/**"]
  docs = ["README.md", "docs/reference/"]

[llm]
  driver = "openai-compatible"
  endpoint = "https://openrouter.ai/api/v1/chat/completions"
  model = "deepseek/deepseek-chat"
  api_key = "${DRIFTLOCK_API_KEY}"
  [llm.options]
    temperature = 0.0
    max_tokens = 4096

[behavior]
  auto_fix = true
  block_on_false = true
  block_on_llm_error = false
  max_retries = 2
  include_full_diff = false
  cache = true

[audit]
  solana = false
  rpc_endpoint = ""
  keypair_path = ""
  program_id = ""
```

See also: [Providers](./providers.md) · [Caching](./caching.md) · [CI/CD](./ci-cd.md).
