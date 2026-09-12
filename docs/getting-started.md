# Getting started

This guide takes you from zero to a working Driftlock installation and your first blocked commit in a few minutes. For a slower, more pedagogical walkthrough, see the [tutorial](./tutorial.md).

## Prerequisites

- **Git** — Driftlock operates on your staged index and on `git` ranges.
- An **LLM endpoint and API key** for a chat-completions-compatible provider (OpenRouter, Groq, DeepSeek, Together, vLLM, or a local Ollama). See [Providers](./providers.md) for the full list and recommended models.
  - Not always required: an added symbol the docs never mention, and a removed symbol they still describe, are decided by string matching with no model call. Set `check_mode = "deterministic"` to run with no provider at all.
- For installing from source: **Go 1.24+**.

## 1. Install

Pick whichever of the following matches your platform.

### Unix (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Ksschkw/driftlock/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/Ksschkw/driftlock/main/install.ps1 | iex
```

### From source (any platform)

```bash
go install github.com/Ksschkw/driftlock/cmd/driftlock@latest
```

`go install` places the `driftlock` binary in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

### Verify the install

```bash
driftlock version
```

If the command runs, Driftlock is on your `PATH`. It prints the release tag, the
commit it was built from, and the Go version; a binary built from a working tree
reports `dev`.

## 2. Initialize your repository

From the root of a Git repository, run the interactive setup:

```bash
driftlock init
```

`driftlock init` walks you through every configuration option (press **Enter** to accept the default shown in `[brackets]`) and then:

1. Writes a complete `.driftlock.toml` at the Git root.
2. Installs the pre-commit hook **where git actually reads it** — `core.hooksPath`
   if you have set it (husky, a shared hooks directory), otherwise
   `.git/hooks/pre-commit`.
   - An existing hook is **never overwritten**. Driftlock's block is appended to
     it, and the original is backed up to `pre-commit.driftlock-backup` first.
   - Running `init` twice changes nothing the second time.
3. Updates `.gitignore`: `.driftlock/` (local cache and audit log) and `.env`
   (secrets) are always ignored. `.driftlock.toml` is ignored **only** if it
   still contains a literal API key; otherwise it stays committable so your team
   can share one policy file.

A typical session looks like this:

```text
  Driftlock interactive setup
  Press Enter to accept the default value shown in [brackets].

── Documentation mapping ──
  Source file patterns (space-separated) [src/**]: src/**
  Documentation files or directories (space-separated) [README.md docs/]: README.md

── LLM provider ──
  Driver (openai-compatible / ollama) [ollama]: openai-compatible
  Full endpoint URL [http://localhost:11434]: https://openrouter.ai/api/v1/chat/completions
  Model name [codestral:22b]: deepseek/deepseek-chat
  API key (or ${ENV_VAR}) []: ${DRIFTLOCK_API_KEY}

── LLM options ──
  Temperature [0]: 0
  Max tokens [2048]: 2048

── Behavior ──
  Auto-fix documentation on drift (y/n) [y]: y
  Block commit when docs are outdated (y/n) [y]: y
  Block commit when LLM is unreachable (y/n) [n]: n
  Max LLM retries [2]: 2
  Send full diff to LLM (uses more tokens) (y/n) [n]: n

── Solana audit (optional) ──
  Enable Solana audit logging (y/n) [n]: n

Installed the pre-commit hook at /path/to/repo/.git/hooks/pre-commit

Driftlock initialized successfully.
The pre-commit hook is active. .driftlock.toml is safe to commit
(it holds no secret) so your team shares one policy; .env and
.driftlock/ are gitignored.
```

> **Note:** `driftlock init` refuses to run if a `.driftlock.toml` already exists. Remove it first if you want to reinitialize.

See [Configuration](./configuration.md) for a full reference of the file it writes.

## 3. Set your API key

Driftlock expands `${ENV_VAR}` references in the `api_key` field of `.driftlock.toml`. The conventional variable is `DRIFTLOCK_API_KEY`:

```toml
[llm]
api_key = "${DRIFTLOCK_API_KEY}"
```

Export it in your shell (or add it to a `.env` you source):

```bash
export DRIFTLOCK_API_KEY="sk-or-v1-your-real-key"
```

> **Tip:** With `${DRIFTLOCK_API_KEY}` in the config, `.driftlock.toml` holds no secret and is meant to be committed, so the whole team shares one policy. If you instead paste a literal key, `init` gitignores the config and prints how to move the key into `.env`. `DRIFTLOCK_API_KEY` is the same variable the [GitHub Action](./ci-cd.md) uses.

## 4. Make your first (blocked) commit

Assume `README.md` documents a function `Login(user string)` and you change its signature in `src/auth.go`:

```go
// before
func Login(user string) error { ... }

// after
func Login(user, password string) error { ... }
```

Stage and commit:

```bash
git add src/auth.go
git commit -m "auth: require password on Login"
```

The pre-commit hook fires. Driftlock detects that the `Login` signature changed but `README.md` still describes the old one, asks the LLM to confirm the drift, rewrites the affected section (because `auto_fix` is on), and **blocks the commit** so you can review the rewrite:

```text
driftlock: README.md → outdated (README still documents Login with a single argument.)
driftlock: README.md has been updated to reflect your changes.

Commit blocked: documentation is out of sync. Review the updated files and stage them.
```

Inspect the diff Driftlock produced, stage it, and commit again:

```bash
git diff README.md      # review the auto-fix
git add README.md
git commit -m "auth: require password on Login"
```

This time the docs match the code, and the commit succeeds.

> **Why that was cheap:** a *modified* signature the docs do mention is the one
> case that genuinely needs a model. A brand-new function nobody documented, or a
> removed one still described, is caught by string matching alone — instantly,
> reproducibly, and with no tokens spent.

### Bypass for a single commit

If you need to commit without running Driftlock (for example, a work-in-progress spike), set `DRIFTLOCK_SKIP` for that one commit:

```bash
DRIFTLOCK_SKIP=true git commit -m "wip: spike, docs to follow"
```

## Next steps

- Follow the full [tutorial](./tutorial.md) for a build-it-yourself walkthrough including `driftlock:ignore` and CI.
- Wire Driftlock into pull requests with [CI/CD](./ci-cd.md).
- Tune costs and pick a model in [Providers](./providers.md) and [Caching](./caching.md).
