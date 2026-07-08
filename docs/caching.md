# The verdict cache

Driftlock caches its drift **verdicts** so that identical checks never re-hit — or re-bill — the LLM. This is what makes Driftlock cheap to run on every commit, every amend, every rebase, and every CI re-run.

## The core idea: verdicts are a pure function

A drift verdict depends on exactly three things:

- the **model** being asked,
- the **structural diff** (the added/removed/modified signatures), and
- the **doc chunk** (the smart-chunked documentation section being checked).

Given the same three inputs, the answer is always the same. That makes the verdict a pure function of `(model, structural-diff, doc-chunk)` — and pure functions are safe to memoize. Driftlock hashes those inputs into a cache key and stores the verdict under it.

## Where it lives

```
.driftlock/cache.json
```

The cache file sits in the `.driftlock/` directory at your Git root. `driftlock init` adds `.driftlock/` to your `.gitignore`, so the cache is **local to each checkout** and never committed. (This is intentional: a cache is a local optimization, not shared state.)

## How a check uses it

For each drifted symbol, Driftlock:

1. Computes the cache key from `(model, structural-diff, doc-chunk)`.
2. **Cache hit** → returns the stored verdict immediately. No network call, no tokens spent.
3. **Cache miss** → asks the LLM, then writes the verdict back under that key for next time.

Because the key is content-addressed, you get automatic reuse across situations that would otherwise repeat work:

- **Commit `--amend`** with the same code and docs → cache hit.
- **Rebase** that replays the same changes → cache hit.
- **CI re-run** of an unchanged PR → cache hit.
- Two developers who happen to produce the identical change → each hits their own local cache after the first run.

## When it invalidates

The key changes — producing a **cache miss** and a fresh LLM call — whenever any input changes:

| Change | Effect |
| --- | --- |
| You edit the **code** (a different signature diff) | New key → re-check. |
| You edit the **documentation** (a different chunk) | New key → re-check. |
| You switch the **model** (different `model` in `[llm]`) | New key → re-check. |

Conversely, changes that do **not** affect the three inputs — reformatting an unrelated file, editing a function body, changing a comment — do not change the key, so a prior verdict is still valid. **Stale verdicts are never served:** if any of the three inputs differ in any way, the key differs, and Driftlock asks again.

## Enabling and disabling

Caching is controlled by `cache` under `[behavior]` and is **on by default**.

```toml
[behavior]
cache = true    # default — omitting the field is the same as true
```

Disable it explicitly if you want every check to hit the LLM (for example, while debugging prompt behavior):

```toml
[behavior]
cache = false
```

To clear the cache manually, delete the file:

```bash
rm .driftlock/cache.json
```

The next run rebuilds it from scratch.

## The economic rationale

Running a documentation check on *every* commit sounds expensive — but with the cache it is not:

- The first time a given `(model, diff, doc)` is seen, you pay for one small LLM call (a structural diff plus a chunked doc section — see [Providers → economics](./providers.md)).
- Every identical check thereafter is **free**. Amend loops, interactive rebases, and CI retries — all common ways the same check runs many times — cost nothing after the first.
- This **bounds** your token spend to roughly "one call per genuinely new `(code-change, doc, model)` combination," rather than "one call per commit attempt."

Combined with structural-only diffs and smart chunking, the cache is why Driftlock can gate every commit without a meaningful cost.

See also: [Configuration → `[behavior]`](./configuration.md) · [Providers → economics](./providers.md) · [Architecture](./architecture.md) · [Troubleshooting → cache staleness](./troubleshooting.md).
