# Driftlock — Hardening & Product Roadmap

This file is the working plan for turning Driftlock from a compelling prototype
into a tool a team can trust as a required gate. It is organised as
**Milestones → Sub-milestones → Micro-milestones**.

Every micro-milestone is a single unit of work that ends in **one commit**.
A micro-milestone is complete only when its acceptance criteria are met and the
tree builds, vets, and tests green.

## Ground rules

- **One commit per micro-milestone.** No bundling.
- **Author:** `Ksschkw <kookafor893@gmail.com>` only. Never add a
  `Co-authored-by:` trailer of any kind, human or AI.
- **Conventional commit messages.** The body explains *what changed and why*,
  names the failure mode being fixed, and lists the tests added.
- **Tests accompany behaviour.** Any change to parser/diff/docman/config/hook
  behaviour ships with a test that fails before and passes after.
- **No silent behaviour changes.** If semantics change, the docs change in the
  same or an immediately following commit.
- **Local hook bypass during work:** `DRIFTLOCK_SKIP=true git commit …`
  (a shell-exported var works; the `.env` path is itself a bug, see M2.2).

## Status legend

`[ ]` todo · `[~]` in progress · `[x]` done · `[!]` blocked / needs a decision

---

## M1 — Parser fidelity: stop missing real code

The parser is the correctness-critical subsystem, and the current extractor
silently misses large classes of mainstream code. A false negative means a
commit with genuinely stale docs passes the gate — the exact failure Driftlock
exists to prevent. **This milestone is the highest priority.**

### M1.A — Language coverage

- [x] **M1.A.1 — Go generic functions.**
  `func Map[T any, U any](xs []T, f func(T) U) []U` currently extracts
  **nothing**, because `pGoFunc` requires `(` immediately after the name.
  Accept an optional type-parameter list after the function name and after a
  generic receiver, and keep the trailing return type in the signature.
  *Accept:* generic funcs, generic methods, and generic receivers are extracted
  with name + full signature; non-generic Go output is byte-identical to before.

- [x] **M1.A.2 — Go generic type declarations.**
  `type Stack[T any] struct` currently extracts nothing (`pGoType` requires
  whitespace immediately after the name). Accept an optional type-parameter
  list before the kind keyword.
  *Accept:* `type Stack[T any] struct`, `type Set[T comparable] map[T]struct{}`.

- [x] **M1.A.3 — TypeScript/JavaScript class methods without modifiers.**
  `class S { run(x: number) { … } }` currently yields only `S`. Add a pattern
  for unmodified class/object methods that requires a body brace, and keep
  control-flow keywords filtered.
  *Accept:* unmodified methods, `get`/`set`, `async`, and `constructor` are
  extracted; `if (…) {`, `for (…) {`, `return foo(a);` are not.

- [x] **M1.A.4 — Explicit capture-group metadata for symbol names.**
  Name extraction guessed: "group 1 is the name unless group 1 looks like a
  type keyword". That made `CREATE TABLE users` a symbol named `TABLE`,
  `def type(x)` a symbol named `x`, and `fn type(x: i32)` a symbol named
  `x: i32`. Replace the guess with a declared name group per pattern.
  *Accept:* SQL/Kotlin/Rust/Python/Markdown names are correct.
- [x] **M1.A.5 — Java/C# methods without access modifiers (package-private).**
  `int add(int a, int b) { … }` currently yields only the class. Add an
  optional-modifier pattern guarded so statement lines (`return foo(a);`,
  `obj.call(x);`) never match.
  *Accept:* package-private and interface methods are extracted; calls and
  `return` statements are not.

- [x] **M1.A.6 — Python return annotations.**
  `-> int` changing to `-> str` currently produces **zero** changes because
  the signature truncates at `)`. Include the return annotation.
  *Accept:* a return-annotation change produces one `modified` change.

- [x] **M1.A.7 — Python signatures must not truncate on brace/semicolon or nested parens.**
  `def f(x: dict = {})` is currently cut to `def f(x: dict = ` because the
  brace/semicolon truncation is applied to every language. Make the truncation
  set language-specific.
  *Accept:* dict/set defaults and `lambda` defaults are preserved verbatim.

- [x] **M1.A.8 — Rust return types.**
  `-> i32` changing to `-> String` currently produces **zero** changes. Include
  the return type (and tolerate `where` clauses).
  *Accept:* a Rust return-type change produces one `modified` change.

- [x] **M1.A.9 — Rust `impl` methods and generic impls.**
  Qualify methods declared inside `impl Type` blocks so same-named methods on
  different types are distinguishable, and accept `impl<T> Trait for Type`.
  *Accept:* two `impl` blocks with a same-named method yield two symbols.

- [x] **M1.A.10 — Kotlin / Swift / Scala return types.**
  Include trailing `: Type` / `-> Type` return annotations in the signature
  where the language declares them inline.
  *Accept:* a return-type change produces one `modified` change per language.

- [x] **M1.A.11 — Rust lifetimes and the `new` keyword.**
  Two independent Rust false negatives found while testing M1.A.9:
  (a) `pub fn get<'a>(x: &'a str) -> &'a str` extracted **nothing**, because the
  sanitizer treats `'` as a char-literal delimiter and blanked the code between
  lifetimes; (b) `fn new()` was dropped as a keyword, so the standard Rust
  constructor was invisible. *Accept:* lifetime-bearing functions extract with
  full signatures; `fn new()` is a symbol; statement guards still apply.

- [x] **M1.A.12 — Public-API filtering.** The parser extracts every
  declaration, including private ones: Python `_private`/`__init__` and Go
  unexported names survive into the structural diff, so an internal refactor can
  trigger a documentation block. The README claims only public signatures are
  considered. *Accept:* Go unexported, Python `_`-prefixed, and non-`pub` Rust
  declarations are excluded from structural changes.
- [x] **M1.A.13 — Go type-expression fidelity.** `type Set[T comparable]
  map[T]struct{}` is captured as `type Set[T comparable] map`, so a change to
  the element type is invisible. *Accept:* the full type expression is part of
  the signature for map/slice/chan/array types.

- [x] **M1.A.14 — PHP return types.** Found while verifying the docs during
  M7.4: `function get(): int` changing to `: string` produced **no** change,
  because pPhpFunc stopped at the parameter list. *Accept:* a PHP return-type
  change registers as one `modified`.

### M1.B — Symbol identity (overloads and same-named methods)

- [x] **M1.B.1 — Dedup by name+signature, not bare name.**
  `parser/universal.go` currently drops every same-named declaration after the
  first (`seen[name]`). Two `Close()` methods on different receiver types
  collapse into one, so deleting one is misreported or missed entirely.
  *Accept:* `A.Close` + `B.Close` produce two signatures.

- [x] **M1.B.2 — Diff as a multiset per name.**
  `diff.ExtractStructuralChanges` keys maps by bare name, so deleting one
  Java overload reports nothing and deleting `A.Close` reports a bogus
  `modified` against `B.Close`. Pair identical signatures first, then report
  unmatched old/new as removed/added, and `modified` when exactly one of each
  remains for a name.
  *Accept:* overload removal → one `removed`; same-name method removal → one
  `removed`; single signature edit → one `modified`.

- [x] **M1.B.3 — Qualified display names.**
  Where cheap (Go receiver, Rust `impl`), show `Type.Method` in the formatted
  diff so the LLM sees a precise symbol. Bare names remain the key for doc
  chunking.
  *Accept:* formatted diff shows qualified names; chunking unchanged.

### M1.C — Evidence and diagnostics

- [x] **M1.C.1 — Golden-corpus harness.**
  A table-driven test that reads `<lang>.<ext>` pairs from `testdata/` and
  compares extracted signatures against a golden file, with an update flag.
  *Accept:* adding a corpus file without a golden fails loudly.

- [x] **M1.C.2 — Regression corpus for every review finding.**
  One test per bug in this section (generics, unmodified methods, return
  types, overloads, dict defaults, same-name methods).
  *Accept:* reverting any fix turns a test red.

- [x] **M1.C.3 — Parse diagnostics.**
  When a mapped source file yields zero signatures but is non-trivial, report
  it (debug channel, and optionally a `--strict` warning) so silence is
  meaningful instead of ambiguous.
  *Accept:* a deliberately unparseable file is reported, not silently skipped.

---

## M2 — Gate correctness: make the verdict trustworthy

### M2.A — Checked content must equal committed content

- [x] **M2.A.1 — Read docs from the git index in staged mode.**
  `hook.go` reads the doc with `os.ReadFile` (working tree) while source content
  comes from HEAD vs the index. Editing a doc without staging it lets the
  commit land a stale doc that Driftlock never checked. Read the staged blob
  (`git show :path`), falling back to disk when unstaged/absent.
  *Accept:* an unstaged doc edit cannot make a drifted commit pass.

### M2.B — Environment and config plumbing

- [x] **M2.B.1 — Load `.env` before the `DRIFTLOCK_SKIP` check.**
  `hook.go` reads `DRIFTLOCK_SKIP` before `config.LoadProjectConfig()` loads
  `.env`, so a `.env`-configured skip silently does nothing. Load dotenv at the
  entry point.
  *Accept:* `DRIFTLOCK_SKIP=true` in `.env` bypasses the hook.

- [x] **M2.B.2 — Deduplicate mapped source files.**
  Overlapping globs push the same source into a doc's list repeatedly, which
  duplicates diff lines and audit entries.
  *Accept:* each source appears once per doc.

### M2.C — Auto-fix honesty

- [x] **M2.C.1 — Only claim a fix when content changed.**
  If the LLM returns empty output or headings fail to merge, Driftlock writes
  an identical file and still prints "has been updated". Compare before/after
  and report honestly.
  *Avoid:* rewriting an unchanged file (mtime churn) and false success.

### M2.D — Timeouts and failure modes

- [x] **M2.D.1 — HTTP client timeout.**
  Adapters use `&http.Client{}` with no timeout and `context.Background()`, so
  a hung provider hangs `git commit` forever. Add a configurable timeout.
- [x] **M2.D.2 — Per-run deadline.**
  Apply a context deadline to the whole run.
- [x] **M2.D.3 — Fail-open with a loud warning.**
  On timeout, respect `block_on_llm_error` but always print an actionable
  message.

---

## M3 — `init` must not be destructive

- [x] **M3.1 — Honor `core.hooksPath`.** Write the hook where git actually
  looks, not blindly to `.git/hooks/`.
- [x] **M3.2 — Chain an existing pre-commit hook.** Append the Driftlock call
  to a foreign hook (husky, lint-staged, pre-commit) instead of overwriting it.
- [x] **M3.3 — Back up before modifying.** Preserve the original as
  `pre-commit.driftlock-backup` with a printed note.
- [x] **M3.4 — Idempotent re-init.** A second `init` must not double-install or
  destroy its own prior work.
- [x] **M3.5 — PATH detection.** Warn when `driftlock` is not resolvable,
  since the hook shells out to it.
- [x] **M3.6 — Tests for every install path** (absent hook, foreign hook,
  husky `hooksPath`, re-init).

---

## M4 — CI and distribution

- [x] **M4.1 — Fix `action.yml` ref defaults.** `default: ${{ github.sha }}`
  in action metadata is a **literal string**; GitHub does not evaluate
  expressions there, so the shipped workflow diffs against a bogus revision.
  Compute base/head inside a `run:` step (or require them explicitly).
- [x] **M4.2 — CI smoke test.** A workflow that runs the action against a known
  drifted fixture and asserts the expected exit code.
- [x] **M4.3 — `driftlock version`.** Build metadata via `-ldflags`.
- [x] **M4.4 — Release hygiene.** Per-asset `.sha256`, correct `checksums.txt`,
  and no build artifacts in the source tree.
- [x] **M4.5 — Strict CI default.** Block on LLM error in CI where silent
  pass-through would defeat the gate.

---

## M5 — Architecture and testability

- [x] **M5.1 — Typed errors instead of `os.Exit`.** `internal/hook` calls
  `os.Exit(1)`, which is why the cache save had to be hand-placed before every
  exit and why the pipeline is untestable. Return `ErrDrift`/`ErrLLM` and let
  `cmd/` choose the exit code.
- [x] **M5.2 — Cache persistence independent of exit order.**
- [x] **M5.3 — Pipeline integration tests** with a fake `Provider` covering
  staged/range, report/json, block paths, and the M2 bugs.
- [x] **M5.4 — Fix or remove `driftlock status`.** It renders a full prompt and
  passes it as the `diff` argument, double-wrapping the check prompt, and reads
  HEAD rather than the working tree.
- [x] **M5.5 — Injectable git layer** so pipeline tests need no real repo.

---

## M6 — Deterministic-first checking (product differentiator)

The LLM is currently used as a coverage checker for the common case, which a
deterministic string check does perfectly, instantly, and for free.

- [x] **M6.1 — Deterministic coverage check** for added/removed symbols.
- [x] **M6.2 — `check_mode = auto|deterministic|llm`** config.
- [x] **M6.3 — Skip the LLM when the deterministic verdict is decisive.**
- [x] **M6.4 — Tests and docs** for the modes and the cost story.

---

## M7 — Housekeeping, correctness details, and docs

- [x] **M7.1 — dotenv inline comments.** `.env.example` ships `KEY=1 # note`,
  but the parser keeps the comment as part of the value, so copying it turns
  debug on permanently.
- [ ] **M7.2 — Fix `.env.example`** to match the parser's real capabilities.
- [x] **M7.3 — Solana honesty.** `~` is never expanded in `keypair_path`, and
  `program_id` is documented but always errors. Implement or clearly mark
  unsupported.
- [x] **M7.4 — Docs sync.** Align README/docs with actual parser coverage
  (return types per language, multi-line generics) and add this roadmap to the
  docs index.
- [ ] **M7.5 — Build artifacts.** Document the release process; keep the tree
  free of ~75 MB of committed-by-accident binaries.
- [ ] **M7.6 — License decision.** BUSL-1.1 with `Change Date: 2099-12-31` is
  off-spec (BUSL caps at four years) and reads as "never open source", which is
  a hard blocker for adoption of a per-repo developer tool. **Needs a
  maintainer decision** — recorded here, not changed unilaterally.

---

## Execution order

1. **M1** (parser fidelity) — without it the product cannot be trusted.
2. **M2** (gate correctness) — the remaining ways a bad commit can slip through.
3. **M3** (init safety) — unblocks first-run adoption.
4. **M4** (CI) — makes the gate enforceable for teams.
5. **M5** (architecture) — unlocks reliable testing and further work.
6. **M6** (deterministic-first) — the cost/speed/trust differentiator.
7. **M7** (housekeeping) — correctness details and docs.

## Progress log

- **M1.A.1** — Go generics + nested-paren parameters. `pGoFunc` now accepts a
  type-parameter list after the function name and a generic receiver, and
  tolerates one level of nested parentheses in parameters. This also fixed a
  pre-existing truncation of func-typed parameters. Tests:
  `internal/parser/go_test.go`.
- **M1.A.2** — Go generic type declarations (`type Stack[T any] struct`,
  `type Set[T comparable] map[T]struct{}`, generic interfaces). Test:
  `TestGoGenericTypeDeclaration`.
- **M1.A.3** — TypeScript/JavaScript methods without modifiers, plus
  multi-modifier methods (`public static`). Added `pTsBareMethod`, made the
  `pTsMethod` modifier group repeatable, and introduced a `;`-free parameter
  list so a declaration can never bind to a later anonymous-function body.
  Tests: `internal/parser/typescript_test.go`.
- **M1.A.4** — Explicit capture-group metadata (`pattern{re, nameGroup}`).
  Fixed misnamed symbols in SQL (`TABLE`→`users`), Python/Rust/Kotlin
  (`type` was read as its parameter list), Go methods named `object`, and
  Markdown (every heading collapsed to `#`). Tests:
  `internal/parser/names_test.go`.
- **M1.A.5** — Java/C# modifier-less methods, interface methods, constructors,
  and C# expression-bodied/auto properties. Added `pJavaBareMethod`,
  `pJavaCtor`, `pCSharpProperty`, and a generic `isStatementStart` guard so
  `return foo(a);` is never read as a declaration. Reused the shared
  `paramListNoSemi`. Tests: `internal/parser/java_test.go`.
- **M1.A.6** — Python return annotations (`-> T`) are now part of the
  signature, so a return-type change registers as one `modified`. Tests:
  `internal/parser/python_test.go`, `TestPythonReturnTypeChangeIsModified`.
- **M1.A.7** — Added `indentDelimited` to the language spec so the C-style
  brace/semicolon cut is skipped for Python/Ruby, and a nested-paren-tolerant
  `pyParamList`. `def f(x: dict = {})` no longer truncates. Test:
  `TestPythonDefaultsWithBracesDoNotTruncate`.
- **M1.A.8** — Rust return types (`-> T`) and `where` clauses are now part of
  the signature, with a closure-tolerant `rustParamList`. Tests:
  `internal/parser/rust_test.go`, `TestRustReturnTypeChangeIsModified`.
- **M1.B.1** — Dedup now keys on `(name, signature)`, so same-named methods on
  different receivers and overloads both survive extraction. Tests:
  `internal/parser/symbols_test.go`.
- **M1.B.2** — `ExtractStructuralChanges` now groups by name and compares
  signatures as multisets, so overload/same-name removal is a real `removed`
  and single edits stay `modified`. Output order is sorted for cache
  stability. Tests: `TestSameNameMethodRemovalIsRemoved`,
  `TestJavaOverloadRemovalIsRemoved`, `TestSameNameMethodEditIsModified`,
  `TestChangeOrderIsDeterministic`.
- **M1.A.10** — Kotlin (`: T`), Swift (`-> T`), and Scala (`: T`) return types
  are part of the signature, with function-typed parameters tolerated. Tests:
  `internal/parser/jvm_test.go`.
- **M2.B.1** — `RunWith` now calls `skipRequested()`, which loads the project
  `.env` before reading `DRIFTLOCK_SKIP`. The repo's own `.env`
  (`DRIFTLOCK_SKIP=true`) previously did nothing. First tests in
  `internal/hook`: `skip_test.go`.
- **M2.A.1** — `readDocForCheck` reads the index blob in staged mode and the
  head-ref blob in range mode, falling back to disk only for untracked docs.
  Added root-aware `git.GetStagedFileContentAt` / `GetFileContentAtRefAt` so
  git runs at the project root rather than the process CWD. Tests:
  `internal/hook/docread_test.go`.
- **M2.B.2** — `ResolveDocMapping` records each source at most once per
  document, so overlapping globs no longer duplicate diff lines or audit
  entries. Tests: `internal/config/docmap_test.go`.
- **M2.C.1** — `mergeFix` reports whether the merged doc actually differs; an
  empty or unmatched reply is a no-op and is reported as needing manual work,
  and no longer rewrites the file. Tests: `internal/hook/fix_test.go`.
- **M2.D.1** — Added `[llm] timeout_seconds` (default 60) and
  `LLMConfig.HTTPTimeout()`, applied to both adapters' `http.Client`. Docs and
  `.driftlock.example.toml` updated. Tests: `internal/config/config_test.go`.
- **M2.D.2** — `checkDocWithRetry` honours context cancellation (no attempt on a
  dead context, interruptible backoff) and each document's check runs under a
  `perCheckBudget` deadline derived from the HTTP timeout and retry count.
  Tests: `internal/hook/retry_test.go`.
- **M2.D.3** — The LLM-failure summary now names the affected docs, states
  whether the commit proceeded UNCHECKED or was blocked, and gives remediation
  (`block_on_llm_error`, `timeout_seconds`, `DRIFTLOCK_DEBUG`). Tests:
  `internal/hook/message_test.go`.
- **M1.A.9** — Rust methods inside `impl` blocks are qualified by type
  (`A.build`, `B.build`) via a new `scopePattern` + `computeScopes` brace
  scan, and generic/trait impls (`impl<T> Trait for Foo<T>`) parse. Tests:
  `TestRustImplMethodsAreScopedByType`, `TestRustGenericTraitImplScope`,
  `TestRustFreeFunctionStaysUnqualified`.
- **M1.A.11** — Rust string delimiters no longer include `'` (it begins a
  lifetime), and statement words (`new`, `delete`, …) are no longer filtered as
  keywords — statement lines are rejected by `isStatementStart` instead. Tests:
  `TestRustLifetimesDoNotBreakExtraction`, `TestRustNewConstructorIsNotFiltered`,
  `TestJavaScriptStatementWordMethodsAreKept`.
- **M1.B.3** — `StructuralChange.Name` (qualified where known) is shown in the
  formatted diff, and chunking name extraction was rewritten: it now handles
  leading modifiers (Java/C#/Rust/TS), Go receivers, and generics, so doc
  chunking no longer degrades to the whole document. Rust `impl` blocks are no
  longer emitted as symbols. Tests: `internal/hook/name_test.go`,
  `TestFormatShowsQualifiedName`.
- **M1.C.1** — Golden-corpus harness (`internal/parser/golden_test.go`) with
  `-update`, plus 10 corpus files (Go, Python, TS, Java, Rust, Kotlin, Swift,
  C#, SQL, YAML) and their goldens. A corpus file without a golden fails
  loudly. Reviewing the generated goldens exposed M1.A.12 and M1.A.13.
- **M1.A.12** — Per-language visibility rules: Go (uppercase = exported),
  Python (leading `_` = private), Rust (`pub` required). Private declarations no
  longer reach the structural diff. Python golden updated by the harness,
  proving it catches the change. Tests: `internal/parser/visibility_test.go`;
  docs/architecture.md documents the rules.
- **M1.C.2** — 12 minimal `regression_*` corpus files, one per finding, plus
  `testdata/README.md` mapping every finding to its sample and asserting test.
  The harness skips `README*` so documentation can live beside the corpus.
- **M1.C.3** — `unparsedSources` reports mapped source files that produce no
  structural signatures, via `[behavior] report_unparsed` (default off) or
  `DRIFTLOCK_DEBUG=1`, and in the JSON report as `unparsed`. Silence is now
  distinguishable from "could not parse". Tests: `internal/hook/unparsed_test.go`.
- **M1.A.13** — Go type expressions run to end of line, with a Go-specific cut:
  struct/interface bodies are dropped (matching the multi-line form) while
  balanced brace groups that are part of the type (`map[T]struct{}`) are kept.
  Trailing comments are trimmed from all signatures. Tests:
  `TestGoTypeExpressionFidelity`, `TestGoStructBodyExcludedFromSignature`,
  `TestGoTypeTrailingCommentExcluded`.
- **M3.1** — `hooksDir` resolves git's `core.hooksPath` (relative to the root,
  or absolute) and falls back to `.git/hooks`. Tests:
  `cmd/driftlock/hooksdir_test.go`.
- **M3.2** — `installPreCommitHook` never overwrites a foreign hook: a fresh
  repo gets a standalone `exec` hook, an existing hook gets a guarded Driftlock
  block appended (no `exec`, so the rest of the hook still runs). Tests:
  `cmd/driftlock/install_test.go`.
- **M3.3** — The original hook is copied to `pre-commit.driftlock-backup` once,
  and a re-run never overwrites that true original. Test:
  `TestInstallHookBacksUpForeignHook`.
- **M3.4** — Install is idempotent: the `# driftlock:begin` marker is detected
  and the file is left byte-for-byte unchanged. Tests:
  `TestInstallHookIsIdempotent`, `TestInstallHookIsIdempotentWithForeignHook`.
- **M3.5** — `driftlockOnPath` warns at install time when the binary the hook
  invokes is not resolvable. Test: `TestDriftlockOnPath`.
- **M3.6** — End-to-end hook tests that execute the installed script against a
  stub binary: the chained hook runs the foreign hook's own commands then
  `driftlock hook-run`, propagates a blocking exit code, and skips cleanly when
  the binary is absent. Tests: `TestChainedHookRunsDriftlock`,
  `TestChainedHookPropagatesFailure`, `TestChainedHookSkipsWhenBinaryMissing`.
- **M4.1** — `base`/`head` action inputs default to empty; refs are resolved in
  a `run:` step (PR base, then push `before`, then `github.sha`) and passed
  through step outputs. Tests: `cmd/driftlock/action_test.go`.
- **M4.2** — End-to-end pipeline tests (`internal/hook/e2e_test.go`) run real git
  plus a stub OpenAI-compatible server against a drifted fixture and assert the
  decision: drift fails, consistent passes, body-only edit never calls the
  model. `.github/workflows/smoke.yml` runs build/vet/test and checks the golden
  corpus is not stale. **This caught a real bug**: in dry-run/range mode an LLM
  error printed an all-clear and passed; `check` now returns an error and the
  summary never claims success after a failure.
- **M4.3** — `driftlock version` prints version/commit/date/Go/os-arch, with
  `-ldflags -X main.*` overrides and a `dev` default for working-tree builds.
  Tests: `cmd/driftlock/version_test.go`; documented in README/docs.
- **M4.4** — `.github/workflows/release.yml` builds all five targets with
  ldflags, publishes per-asset `.sha256` plus `checksums.txt`, and verifies its
  own checksums before upload. Both installers now **abort** on a mismatched
  checksum instead of installing anyway, and `install.sh` no longer dies on a
  missing asset line under `set -e`. Tests: `cmd/driftlock/release_test.go`;
  process documented in `docs/releasing.md`. `dist/` is gitignored.
- **M4.5** — `DRIFTLOCK_STRICT_LLM=1` forces `block_on_llm_error` and
  `block_on_false` on, overriding the config; the action defaults to setting it
  via a new `block-on-llm-error` input. End-to-end tests prove strict mode
  overrides a lenient config while the lenient config still passes without it.
  Tests: `internal/hook/strict_test.go`, `TestEndToEndStrictLLMOverridesLenientConfig`,
  `TestEndToEndLenientConfigAllowsProviderError`; documented in README and
  docs/ci-cd.md.
- **M5.1** — `internal/hook` no longer calls `os.Exit`; it returns sentinel
  `ErrDrift` / `ErrLLMUnreachable` and lets `cmd/` choose the exit code, so
  defers run and the pipeline is testable. Tests assert with `errors.Is`.
- **M5.2** — The verdict cache is saved by a `defer`, so it persists on every
  return path including a blocked commit. Test:
  `TestEndToEndVerdictCacheSurvivesBlockedCommit` asserts `cache.json` exists
  after a drift block.
- **M5.3** — Pipeline tests for the paths the e2e suite did not cover: staged
  mode blocks; staged auto-fix rewrites the doc, preserves the heading, and
  still blocks for review; report mode never fails; and the JSON report is valid
  JSON describing drift, mode, per-doc status, and changes. Tests:
  `internal/hook/pipeline_test.go`.
- **M5.4** — `driftlock status` is now a **deterministic coverage report**: no
  provider, no double-wrapped prompt, no HEAD-vs-working-tree confusion. It
  lists symbols the mapped docs never mention; `--strict` exits non-zero.
  Tests: `internal/hook/status_test.go`; README/docs updated.
- **M5.5** — `Options.source` (unexported) injects a `changeSource` abstraction
  over git, so the pipeline runs entirely in memory in tests. New tests cover an
  injected drifted run, a deleted source (skipped, no model call), and an
  unparsable source reported in the JSON `unparsed` field. Tests:
  `internal/hook/source_test.go`.
- **M6.1** — `deterministicVerdict` decides added/removed symbols by string
  matching (and modified ones when undocumented), returning `(ok, decisive,
  reason)` so the caller knows when a model is genuinely needed. Not yet wired
  into the pipeline. Tests: `internal/hook/decide_test.go`.
- **M6.2** — `[behavior] check_mode` (`auto` default, `deterministic`, `llm`)
  with `ResolvedCheckMode()`; an unset or misspelled value resolves to `auto`
  so a typo cannot silently disable the model. Tests:
  `TestResolvedCheckMode`, `TestLoadConfigCheckMode`.
- **M6.3** — The pipeline consults the deterministic verdict first and only
  falls back to the model when it is not decisive; `check_mode=llm` always calls
  the model and `deterministic` never does. The provider is now created lazily,
  so deterministic runs need no LLM configuration at all. Tests:
  `internal/hook/deterministic_pipeline_test.go` (auto skips for added, uses the
  model for modified; deterministic works with no `[llm]` section; llm always
  calls the model).
- **M1.A.14** — PHP return types (`: int`, `: ?string`, `: int|string`) are now
  part of the signature, found while verifying the docs during M7.4. Tests:
  `internal/parser/php_test.go`, `TestPhpReturnTypeChangeIsModified`; corpus
  sample `regression_php_returns.php`.
- **M6.4** — `check_mode` documented in README, docs/configuration.md,
  docs/architecture.md (new pipeline stage + invariant), docs/caching.md (a
  "deterministic-first" cost section), and `.driftlock.example.toml`.
  `TestExampleConfigLoads` guards the example from rot.
- **M7.1** — `parseDotEnvValue` strips trailing `#` comments (space-preceded)
  while preserving quoted values and `abc#def`. Tests:
  `TestParseDotEnvValue`, `TestLoadDotEnvStripsInlineComments`.
- **M7.2** — `DRIFTLOCK_DEBUG` now uses a truthy check
  (`config.DebugEnabled()`), so the documented `=0` actually disables it; the
  non-empty check had made `0` enable debug. `.env.example` rewritten to match,
  and it now documents `DRIFTLOCK_STRICT_LLM`. Tests: `TestDebugEnabled`,
  `TestExampleDotEnvValues`.
- **M7.3** — Solana: `~` is expanded in `keypair_path` (with a clear error
  naming the resolved path), an empty `keypair_path`/`rpc_endpoint` is reported
  explicitly, the submission is bounded by a 30s timeout, and `program_id` is
  refused with an explanation that only Memo is supported. Tests:
  `internal/audit/solana_test.go`; docs updated.
- **M7.4** — Docs aligned with real parser coverage: per-language return types,
  Go generics/type expressions, visibility filtering, and the `report_unparsed`
  diagnostic. `docs/index.md` links this roadmap. Verifying the claim uncovered
  and fixed M1.A.14 (PHP return types).
