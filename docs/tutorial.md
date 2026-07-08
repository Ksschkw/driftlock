# Tutorial: Driftlock end to end

This is the flagship, hands-on tutorial. By the end you will have:

1. Built a tiny sample repository from scratch.
2. Configured Driftlock with `.driftlock.toml`.
3. Made a real signature change, watched the commit get **blocked**, reviewed the **auto-fix**, and committed cleanly.
4. Used the `driftlock:ignore` annotation to keep an internal helper out of the check.
5. Run Driftlock the way CI runs it, with `driftlock check --base`.

You only need Git, a shell, and the `driftlock` binary on your `PATH` (see [Getting started](./getting-started.md) to install), plus an LLM API key.

Throughout, terminal transcripts show `$` for commands you type. Output is realistic but your LLM's exact wording will vary.

---

## Scenario 1 — Catch and auto-fix a drifted signature

### Step 1.1 — Create the sample repository

```bash
$ mkdir drift-demo && cd drift-demo
$ git init
Initialized empty Git repository in /home/you/drift-demo/.git/
```

Create a source directory and a tiny "calculator" module:

```bash
$ mkdir src
```

`src/calc.go`:

```go
package calc

// Add returns the sum of two integers.
func Add(a int, b int) int {
	return a + b
}
```

And a `README.md` that documents it:

````markdown
# calc

A tiny calculator library.

## API

### Add

```go
func Add(a int, b int) int
```

Returns the sum of `a` and `b`.
````

Commit the starting point so we have a clean baseline:

```bash
$ git add src/calc.go README.md
$ git commit -m "initial calc module"
[main (root-commit) 9f1c2ab] initial calc module
 2 files changed, 12 insertions(+)
```

### Step 1.2 — Initialize Driftlock

Run the interactive setup. For this tutorial we map `src/**` to `README.md` and use an OpenAI-compatible provider (OpenRouter here — see [Providers](./providers.md) for others):

```bash
$ driftlock init

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

The resulting `.driftlock.toml`:

```toml
[[doc_mapping]]
  sources = ["src/**"]
  docs = ["README.md"]

[llm]
  driver = "openai-compatible"
  endpoint = "https://openrouter.ai/api/v1/chat/completions"
  model = "deepseek/deepseek-chat"
  api_key = "${DRIFTLOCK_API_KEY}"
  [llm.options]
    temperature = 0.0
    max_tokens = 2048

[behavior]
  auto_fix = true
  block_on_false = true
  max_retries = 2
  include_full_diff = false
  block_on_llm_error = false

[audit]
  solana = false
```

Export your key:

```bash
$ export DRIFTLOCK_API_KEY="sk-or-v1-your-real-key"
```

### Step 1.3 — Change a signature

Edit `src/calc.go` to add a third parameter — a real change to the public API surface:

```go
package calc

// Add returns the sum of the given integers.
func Add(a int, b int, c int) int {
	return a + b + c
}
```

Notice that `README.md` still documents the two-argument form. That is drift.

### Step 1.4 — Watch the commit get blocked

```bash
$ git add src/calc.go
$ git commit -m "calc: Add now takes three integers"

driftlock: checking staged changes...

  src/calc.go → README.md
    modified: func Add(a int, b int, c int) int
    verdict:  FALSE — README documents Add with two parameters, but the
              signature now takes three (a, b, c).

  auto_fix is on: Driftlock rewrote README.md to match the new signature.
  Review the changes, stage README.md, and commit again.

Commit blocked.
```

Nothing was committed. Driftlock parsed the old and new content of `src/calc.go`, diffed the signatures **by name**, found `Add` modified, mapped `src/calc.go` to `README.md`, extracted just the section mentioning `Add` ("smart chunking"), and asked the LLM whether the docs still match. The verdict was `FALSE`, so — because `auto_fix = true` — it rewrote that section and blocked the commit for your review.

### Step 1.5 — Review the auto-fix

```bash
$ git diff README.md
```

```diff
 ### Add

 ```go
-func Add(a int, b int) int
+func Add(a int, b int, c int) int
 ```

-Returns the sum of `a` and `b`.
+Returns the sum of `a`, `b`, and `c`.
```

Only the `Add` section changed — the rewritten chunk was stitched back into the full document by exact heading match, so the rest of `README.md` is untouched. See [Architecture](./architecture.md) for how chunk-in / stitch-out works.

### Step 1.6 — Stage the fix and commit cleanly

```bash
$ git add README.md
$ git commit -m "calc: Add now takes three integers"

driftlock: checking staged changes...

  src/calc.go → README.md
    modified: func Add(a int, b int, c int) int
    verdict:  TRUE — documentation matches the new signature.

All documentation is in sync.
[main 4b7d10e] calc: Add now takes three integers
 2 files changed, 4 insertions(+), 3 deletions(-)
```

The second check hits the LLM again only if the (model, diff, doc) tuple changed — which it did, because you edited the doc. See [Caching](./caching.md).

> **What did NOT trigger?** If you had only edited the *body* of `Add` (say, `return a + b + c` → `return c + b + a`), changed a comment, or reformatted whitespace, Driftlock would have found no signature change and let the commit through untouched. Only the public API surface triggers a check.

---

## Scenario 2 — Exclude an internal helper with `driftlock:ignore`

Suppose you add an unexported helper that you deliberately do *not* want to document, plus an exported function you *do*. Driftlock only triggers on **exported** symbols, but sometimes you want to suppress a specific declaration explicitly — for example, an exported-but-internal function, or to silence a symbol you have decided not to document. That is what `driftlock:ignore` is for.

Edit `src/calc.go`:

```go
package calc

// Add returns the sum of the given integers.
func Add(a int, b int, c int) int {
	return sum(a, b, c)
}

// Sum is an exported helper we do NOT want tracked in the docs.
// driftlock:ignore
func Sum(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

func Debug(v int) int { return v * -1 } // driftlock:ignore
```

Two forms of the annotation are shown, and **both work in any language's comment syntax**:

- **Standalone** — a line whose only content is the comment `// driftlock:ignore`, placed directly above a declaration. It ignores the declaration on the next line (`Sum` above).
- **Inline** — the marker in a trailing comment on the same line as the declaration (`Debug` above). It ignores only that declaration.

Commit it:

```bash
$ git add src/calc.go
$ git commit -m "calc: add ignored helpers"

driftlock: checking staged changes...

  src/calc.go → README.md
    added: func Add(...)  (unchanged)
    Sum and Debug suppressed by driftlock:ignore.

All documentation is in sync.
[main a19f77c] calc: add ignored helpers
 1 file changed, 15 insertions(+)
```

`Sum` and `Debug` never entered the signature set, so no drift was reported and no doc rewrite happened. See [Ignoring symbols](./ignoring.md) for more examples across languages and for scoping `doc_mapping` so unrelated files never trigger a check in the first place.

---

## Scenario 3 — The CI check with `driftlock check --base`

CI does not have a staging index — it compares two commits. `driftlock check` in **range mode** does exactly that, and it **never writes files** (no auto-fix in CI; the check either passes or fails).

Let's simulate a pull request locally. Create a feature branch and drift the docs again:

```bash
$ git checkout -b feature/multiply
Switched to a new branch 'feature/multiply'
```

Add a new exported function to `src/calc.go` without documenting it:

```go
// Multiply returns the product of two integers.
func Multiply(a int, b int) int {
	return a * b
}
```

Because auto-fix runs in the pre-commit hook, commit with a skip so we get a *drifted* commit onto the branch to test CI behavior (in real life a teammate might commit past the hook, or edit code on the web UI):

```bash
$ git add src/calc.go
$ DRIFTLOCK_SKIP=true git commit -m "calc: add Multiply (docs pending)"
[feature/multiply 7c2a5e1] calc: add Multiply (docs pending)
 1 file changed, 5 insertions(+)
```

Now run the check exactly as CI would — comparing the branch tip against `main`:

```bash
$ driftlock check --base main --head HEAD

driftlock: comparing main..HEAD

  src/calc.go → README.md
    added: func Multiply(a int, b int) int
    verdict: FALSE — README does not document Multiply.

Documentation is out of sync.

$ echo $?
1
```

`driftlock check` exited **non-zero**, which is what fails the CI job. Note it did **not** modify `README.md` — `check` is read-only by design.

### Report-only mode (gradual rollout)

While you are still bringing existing docs into sync, you may not want CI to be red on day one. Add `--report` to always exit `0` while still printing the findings:

```bash
$ driftlock check --base main --head HEAD --report

  src/calc.go → README.md
    added: func Multiply(a int, b int) int
    verdict: FALSE — README does not document Multiply.

Documentation is out of sync (report-only; not failing).

$ echo $?
0
```

### Machine-readable output

Add `--json` to emit a report to stdout for other tools to consume:

```bash
$ driftlock check --base main --head HEAD --json
```

```json
{
  "in_sync": false,
  "results": [
    {
      "source": "src/calc.go",
      "doc": "README.md",
      "changes": [
        { "kind": "added", "signature": "func Multiply(a int, b int) int" }
      ],
      "verdict": false,
      "reason": "README does not document Multiply."
    }
  ]
}
```

> The exact JSON shape is produced by the tool; treat the fields above as illustrative and inspect real output in your pipeline.

### Fix it and go green

Document `Multiply` in `README.md`, or let the local hook do it for you by running `driftlock fix` (which force-regenerates all mapped docs for staged files) and committing:

```bash
$ git add src/calc.go
$ driftlock fix           # regenerate mapped docs for staged files
$ git add README.md
$ git commit -m "calc: document Multiply"
```

Re-run the check:

```bash
$ driftlock check --base main --head HEAD
driftlock: comparing main..HEAD
All documentation is in sync.
$ echo $?
0
```

Green. To wire this into an actual GitHub pull request, see [CI/CD](./ci-cd.md) — including the crucial `fetch-depth: 0` checkout setting so both refs exist in CI.

---

## Recap

- Driftlock triggers only on **structural** (exported signature) changes, never on bodies, comments, or formatting.
- The pre-commit hook can **auto-fix** and blocks the commit for your review; `driftlock check` in CI is **read-only** and just passes or fails.
- `driftlock:ignore` (inline or standalone) removes a declaration from analysis.
- `--report` turns a failing check into a passing, informational one for gradual adoption; `--json` makes it machine-readable.
- Repeated identical checks are cached and cost nothing extra ([Caching](./caching.md)).

Continue with [Configuration](./configuration.md) to fine-tune behavior, or [Providers](./providers.md) to pick a cheaper/faster model.
