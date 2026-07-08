# Getting started

This guide takes you from zero to a working Driftlock installation and your first blocked commit in a few minutes. For a slower, more pedagogical walkthrough, see the [tutorial](./tutorial.md).

## Prerequisites

- **Git** — Driftlock operates on your staged index and on `git` ranges.
- An **LLM endpoint and API key** for a chat-completions-compatible provider (OpenRouter, Groq, DeepSeek, Together, vLLM, or a local Ollama). See [Providers](./providers.md) for the full list and recommended models.
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
driftlock status
```

If the command runs, Driftlock is on your `PATH`.

## 2. Initialize your repository

From the root of a Git repository, run the interactive setup:

```bash
driftlock init
```

`driftlock init` walks you through every configuration option (press **Enter** to accept the default shown in `[brackets]`) and then:

1. Writes a complete `.driftlock.toml` at the Git root.
2. Installs `.git/hooks/pre-commit` (which simply runs `driftlock hook-run`).
3. Adds `.driftlock.toml` and `.driftlock/` to your `.gitignore`.

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

Driftlock initialized successfully.
A .driftlock.toml has been created, the pre-commit hook is active,
and .driftlock.toml and .driftlock/ have been added to .gitignore.
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

> **Tip:** Because `.driftlock.toml` is git-ignored by `init`, you *can* paste a literal key into it, but using `${DRIFTLOCK_API_KEY}` keeps secrets out of files entirely. This is the same variable the [GitHub Action](./ci-cd.md) uses.

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
driftlock: documentation is out of sync with structural code changes

  src/auth.go → README.md
    modified: func Login(user, password string) error
    reason: README still documents Login with a single argument.

  Driftlock rewrote README.md to match. Review the changes, then:
    git add README.md
    git commit

Commit blocked.
```

Inspect the diff Driftlock produced, stage it, and commit again:

```bash
git diff README.md      # review the auto-fix
git add README.md
git commit -m "auth: require password on Login"
```

This time the docs match the code, and the commit succeeds.

### Bypass for a single commit

If you need to commit without running Driftlock (for example, a work-in-progress spike), set `DRIFTLOCK_SKIP` for that one commit:

```bash
DRIFTLOCK_SKIP=true git commit -m "wip: spike, docs to follow"
```

## Next steps

- Follow the full [tutorial](./tutorial.md) for a build-it-yourself walkthrough including `driftlock:ignore` and CI.
- Wire Driftlock into pull requests with [CI/CD](./ci-cd.md).
- Tune costs and pick a model in [Providers](./providers.md) and [Caching](./caching.md).
