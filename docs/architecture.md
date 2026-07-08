# Architecture

This page describes how Driftlock works end to end, the package layout, and the design invariants that keep it correct and cheap.

## The pipeline

Every check — whether triggered locally by the pre-commit hook or in CI by `driftlock check --base` — flows through the same pipeline:

```
  git (staged index, or base..head)
        │
        ▼
  parse old vs new content into signatures      (per-language, comment-stripped)
        │
        ▼
  diff signatures BY NAME → added / removed / modified
        │
        ▼
  map changed source files → docs               (via [[doc_mapping]])
        │
        ▼
  smart chunking: extract only the doc sections
  mentioning the changed symbol names
        │
        ▼
  check cache  ──hit──▶  reuse verdict (no LLM call)
        │ miss
        ▼
  ask LLM: Check → TRUE/FALSE + one-sentence reason
        │
        ▼
  if outdated AND auto_fix:
     LLM rewrites the chunked sections
     → merge back into the full doc by EXACT HEADING MATCH
     → block the commit so the author can review & stage
        │
        ▼
  append SHA-256 of (diff + doc) to .driftlock/audit.jsonl
  (optionally anchor the hash on Solana)
```

### Stage by stage

1. **Source of truth: git.** Locally, Driftlock reads the **staged index** — the exact content being committed. In CI, it reads the two endpoints of a range, `base..head`. Either way it obtains the *old* and *new* content of each changed file.

2. **Parse into signatures.** Each file's content is parsed into a set of public signatures (functions, methods, types, classes, interfaces, structs). Parsing is **dispatched by file extension** and runs on **comment-and-string-stripped** content (see [Parser](#parser) below).

3. **Diff by name.** The old and new signature sets are diffed **keyed by symbol name**, yielding `added`, `removed`, and `modified` sets. A rename therefore appears as one `removed` (old name) plus one `added` (new name).

4. **Map to docs.** Each changed source file is matched against `[[doc_mapping]]` `sources` globs; the associated `docs` (files, or directories expanded to their `*.md` files) become the documents to verify.

5. **Smart chunking.** Rather than sending an entire Markdown file, Driftlock extracts only the **section(s) that mention the changed symbol names**. This is the "chunk-in" half of the round trip and the main cost saver.

6. **Cache check.** The `(model, structural-diff, doc-chunk)` tuple is hashed into a cache key. A hit reuses the stored verdict with no LLM call. See [Caching](./caching.md).

7. **LLM Check.** On a miss, the model is asked whether the doc chunk still reflects the changes. It answers `TRUE` (in sync) or `FALSE` (drifted) plus a one-sentence reason. The verdict is cached.

8. **Auto-fix (optional).** If the verdict is `FALSE` and `auto_fix` is on, the LLM **rewrites the chunked sections**. The rewritten chunks are the "stitch-out" half: they are merged back into the full document by **exact heading match**, leaving every other section byte-for-byte unchanged. The commit is then **blocked** so the author can review and stage the rewrite — Driftlock never silently commits generated prose.

9. **Audit.** A SHA-256 of `(diff + doc)` is appended to `.driftlock/audit.jsonl`. If Solana auditing is enabled, the hash is also anchored on-chain.

> In CI (`driftlock check`), the pipeline is **read-only**: steps 1–7 run, but nothing is written — no auto-fix, no file changes. The check simply passes or fails (exits non-zero on drift, unless `--report`). See [CI/CD](./ci-cd.md).

---

## Package layout

Driftlock is a Go program. The command entry points live under `cmd/driftlock`; the implementation lives under `internal/`.

```
cmd/driftlock/            CLI commands (Cobra)
  main.go, root.go        wiring
  init.go                 interactive setup, hook + .gitignore install
  hook_run.go             internal command run by the pre-commit hook
  check.go                read-only check (staged or base..head; --report/--json)
  fix.go                  force-regenerate mapped docs for staged files
  log.go                  show the last 20 audit entries
  status.go               show status

internal/
  git/                    read staged index and base..head content
  config/                 parse .driftlock.toml, defaults, glob matching, doc mapping
  parser/                 per-language signature extraction (dispatch, sanitize, universal)
  diff/                   name-keyed signature diffing → added/removed/modified
  docman/                 smart chunking + exact-heading stitch-back of rewritten docs
  llm/                    provider interface + adapters (OpenAI-compatible, Ollama)
  cache/                  content-addressed verdict cache (.driftlock/cache.json)
  audit/                  SHA-256 audit log (.driftlock/audit.jsonl) + Solana anchoring
  hook/                   orchestrates the pipeline for the hook and for check
```

Responsibilities:

- **`internal/git`** — extracts old/new file content from the staging index or a commit range.
- **`internal/config`** — loads and defaults `.driftlock.toml`, expands `${ENV_VAR}`, and resolves `[[doc_mapping]]` globs (`*`, `**`, `?`).
- **`internal/parser`** — turns file content into signatures; extension dispatch, comment/string sanitization, `driftlock:ignore`, and a universal fallback.
- **`internal/diff`** — diffs old vs new signatures by name.
- **`internal/docman`** — the "chunk-in / stitch-out" doc manager: extracts sections mentioning changed symbols and merges rewrites back by exact heading.
- **`internal/llm`** — the `Provider` interface with `Check` and `Fix`, plus the OpenAI-compatible and Ollama adapters.
- **`internal/cache`** — hashes `(model, diff, doc)` and stores verdicts.
- **`internal/audit`** — hashes `(diff + doc)`, appends to the local JSONL log, and optionally writes to Solana.
- **`internal/hook`** — the orchestrator that runs the whole pipeline for both `hook-run` and `check`.

---

<a id="parser"></a>
## The parser

Driftlock parses **any language** through per-language regex extractors, with three properties that keep it accurate:

1. **Language-dispatched by extension.** A `.go` file runs only Go patterns; it never runs YAML or Markdown patterns, and vice versa. Unknown extensions fall back to a **conservative universal code spec**.
2. **Comment- and string-stripped first.** Before any pattern runs, comments and string literals are removed, so a signature-looking token inside a comment or a string never produces a phantom signature.
3. **Multi-line aware.** Signatures spread across several lines (long parameter lists, multi-line generics) are matched as a unit.

Supported languages include Go, Python, JavaScript/TypeScript, Java, C#, C/C++, Rust, Swift, Kotlin, Scala, PHP, Ruby, Shell/Bash, Lua, Clojure, and SQL (`CREATE TABLE`/`VIEW`/…), plus data and markup formats: YAML, JSON, TOML/INI, XML/HTML, and Markdown.

The `driftlock:ignore` marker (inline or standalone) removes a declaration from the extracted set — see [Ignoring symbols](./ignoring.md).

---

## Key design invariants

These invariants are the load-bearing guarantees of the system:

- **Name-keyed diffing ⇒ rename = remove + add.** Signatures are diffed by symbol name, so renaming an exported symbol is reported as one removed and one added signature — never a silent "modify."

- **Chunk-in / stitch-out with exact-heading merge.** Only the doc sections mentioning changed symbols are sent to the LLM, and rewritten sections are merged back into the full document by **exact heading match**. Untouched sections are preserved verbatim. This bounds cost and prevents the LLM from rewriting parts of a doc it was never asked about.

- **Language-dispatched, comment-stripped parsing.** Patterns are chosen by extension and run only over sanitized (comment/string-free) content, so drift is judged on the real API surface — never on tokens hiding in comments or string literals.

- **Content-addressed caching.** Verdicts are memoized on `(model, structural-diff, doc-chunk)`. Identical checks never re-bill the LLM, and any change to code, docs, or model invalidates the key so stale verdicts are never served. See [Caching](./caching.md).

- **Auto-fix never commits silently.** When Driftlock rewrites docs, it **blocks the commit** so a human reviews and stages the change. In CI the pipeline is read-only and only reports pass/fail.

See also: [Caching](./caching.md) · [Ignoring symbols](./ignoring.md) · [CI/CD](./ci-cd.md) · [Configuration](./configuration.md).
