# Troubleshooting

Practical fixes for the situations you are most likely to hit. If something here
doesn't cover your case, run with [`DRIFTLOCK_DEBUG=1`](#see-exactly-what-the-llm-received)
and inspect the payloads.

## A commit was blocked and I don't understand why

Driftlock blocks a commit only when **all** of the following are true:

1. A staged file matches a `[[doc_mapping]]` `sources` glob.
2. That file has a **structural** change — an added, removed, or modified
   public function / method / type / class / interface / struct signature.
   Body edits, comments, and formatting never trigger it.
3. The LLM judged the mapped documentation to be **out of sync** with that
   change (`block_on_false = true`, the default).

Read the per-document line Driftlock printed — it names the doc and gives a
one-sentence reason, e.g.:

```
driftlock: README.md → outdated (the Connect signature gained a timeout argument not shown in the docs)
```

Your options:

- Let the auto-fix do the work (default): Driftlock rewrote the doc and blocked
  the commit so you can review. Inspect the diff, then `git add` the doc and
  commit again.
- Fix the doc yourself, `git add` it, and re-commit.
- Decide the symbol is not part of your public contract and exclude it with a
  [`driftlock:ignore`](./ignoring.md) annotation or a narrower `doc_mapping`.
- Bypass this one commit (see below).

## I need to commit right now and bypass the check

```bash
DRIFTLOCK_SKIP=true git commit -m "wip: infra change, docs to follow"
```

`DRIFTLOCK_SKIP=true` makes the hook exit immediately without checking anything.
Use it sparingly — it is an escape hatch, not a workflow.

## The LLM is unreachable / I keep getting LLM errors

You will see a yellow warning like:

```
driftlock: README.md → LLM error: all retries exhausted: LLM request failed: ...
```

- By default (`block_on_llm_error = false`), the commit is **allowed** with the
  warning, so a provider outage never stops your work.
- If you set `block_on_llm_error = true` (common in CI), the commit/check is
  **blocked** when the LLM can't be reached.

Checklist:

- Is your API key set? `echo $DRIFTLOCK_API_KEY` should be non-empty when your
  config uses `api_key = "${DRIFTLOCK_API_KEY}"`.
- Is `endpoint` the **full** chat-completions URL? For OpenRouter it must be
  `https://openrouter.ai/api/v1/chat/completions`, not just the host. See
  [Providers](./providers.md).
- Is `model` spelled exactly as the provider expects?
- Run with `DRIFTLOCK_DEBUG=1` to see the exact request and the provider's error
  body.

## Driftlock missed a symbol / flagged something I didn't expect

Driftlock's parser dispatches by file extension, strips comments and strings,
and matches public (exported) signatures. A few consequences:

- **A symbol wasn't detected.** Confirm the file's extension is recognized (see
  the language list in [Architecture](./architecture.md)). Unknown extensions
  fall back to a conservative universal parser that may not catch exotic
  syntax. For lowercase-public languages (Python, Rust) a leading underscore
  marks a symbol private and it is skipped.
- **A symbol was flagged that you consider internal.** Exclude it with a
  [`driftlock:ignore`](./ignoring.md) annotation, or scope your `doc_mapping`
  `sources` so that file isn't watched.
- **A rename looks like two changes.** That's expected: signatures are diffed by
  name, so a rename is reported as one *removed* and one *added* symbol.

## CI can't find the base ref

Symptom in the GitHub Action or a raw `driftlock check --base ...` run:

```
failed to list changed files in range: git diff <base>..<head> failed: ... unknown revision
```

The runner's checkout is shallow, so the base commit isn't present locally. Fix
it by fetching full history in the checkout step:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

See [CI/CD](./ci-cd.md) for the complete workflow.

## CI mode never blocks / always passes

- Confirm you are not passing `--report` (report mode always exits `0`).
- Confirm `[[doc_mapping]]` `sources` actually match the files your PR changed —
  a mapping that matches nothing produces "Nothing to check" and passes.
- Confirm `.driftlock.toml` exists in the `working-directory` you pointed the
  action at.

## A verdict seems stale after I changed the docs

The [verdict cache](./caching.md) is keyed by `(model, structural-diff,
doc-chunk)`. Any change to the code change or the documentation produces a new
key, so a genuinely stale hit should not happen. If you suspect the cache:

- Delete it: `rm .driftlock/cache.json`. It is rebuilt on the next run.
- Or disable it entirely with `cache = false` under `[behavior]`.

## Auto-fix rewrote the wrong thing / mangled a heading

Rewritten chunks are merged back into the full document by **exact heading
match**. If your custom `[llm.prompts].fix` prompt lets the model alter a
heading's text or level, that section won't merge cleanly. Ensure any custom
fix prompt instructs the model to **preserve every heading exactly**. The
built-in prompt already does this.

## See exactly what the LLM received

```bash
DRIFTLOCK_DEBUG=1 git commit -m "..."      # local hook
DRIFTLOCK_DEBUG=1 driftlock check          # manual
```

This prints, to stderr:

- the chunked documentation length vs. the full document length,
- the full request body sent to the LLM,
- the raw response, and
- token usage (`prompt`, `completion`, `total`) when the provider reports it.

It's the fastest way to debug prompt behavior and to understand your token
spend.

## Getting help

- Review the [Configuration reference](./configuration.md) for exact field
  semantics and defaults.
- Review [Architecture](./architecture.md) to understand what triggers a check.
- Open an issue at <https://github.com/Ksschkw/driftlock/issues>.
