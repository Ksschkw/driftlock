# Ignoring symbols and scoping checks

Sometimes a signature change should *not* require a documentation update — an internal helper, a deliberately undocumented experimental API, or a symbol you have decided lives outside your public contract. Driftlock gives you two complementary controls:

1. The **`driftlock:ignore`** annotation, to exclude a specific declaration.
2. **Scoping `[[doc_mapping]]`**, to keep whole files or trees out of analysis in the first place.

## What triggers a check (recap)

Driftlock triggers only on **structural changes to the public (exported) API surface**: added, removed, or modified function, method, type, class, interface, or struct signatures. It does **not** trigger on:

- Function/method **body** edits.
- **Comment** changes.
- **Whitespace / formatting** changes.
- Renaming a **local variable**.

A rename of an exported symbol shows up as one removed signature plus one added signature.

If a symbol is triggering a check that you do not want, use one of the tools below.

---

## The `driftlock:ignore` annotation

Place the marker `driftlock:ignore` in a **comment** to exclude a declaration from analysis. It works in **any language's comment syntax** — Driftlock strips comments before matching signatures, and recognizes the marker in whatever comment form the language uses.

There are two placements, and **both work**:

### 1. Inline — same line as the declaration

The marker in a trailing comment ignores **only that declaration**:

```go
func Internal() {} // driftlock:ignore
```

### 2. Standalone — a comment line directly above the declaration

A line whose **only content** is the comment `driftlock:ignore`, immediately followed by the declaration on the next line, ignores **that next declaration**:

```go
// driftlock:ignore
func Internal() {}
```

### Examples across languages

Because the marker is matched inside comments, the same idea applies everywhere. Use each language's own comment delimiter.

**Go / Java / C# / C++ / Rust / Swift / Kotlin / Scala / TypeScript** (`//`):

```go
// driftlock:ignore
func mustNotDocument() {}

func alsoIgnored() {} // driftlock:ignore
```

**Python / Ruby / Shell / YAML** (`#`):

```python
# driftlock:ignore
def internal_helper():
    ...

def another():  # driftlock:ignore
    ...
```

**C / C++ block comment** (`/* ... */`):

```c
/* driftlock:ignore */
int internal_thing(void) { return 0; }
```

**Lua** (`--`):

```lua
-- driftlock:ignore
function M.internal() end
```

**PHP** (`//` or `#`):

```php
// driftlock:ignore
function internalHelper() {}
```

> The marker's text is exactly `driftlock:ignore`. For the **standalone** form, the comment line must contain only the marker (no other words), and the declaration must be on the **next** line. For the **inline** form, put the marker in the trailing comment on the declaration line.

### When to reach for it

- An exported symbol that is genuinely internal or experimental and you have chosen not to document.
- Generated code you do not want Driftlock to police.
- Silencing a single noisy declaration while you decide how to document it.

---

## Scoping `[[doc_mapping]]` to avoid over-triggering

The annotation excludes individual declarations. `[[doc_mapping]]` decides which files are even considered — the cheaper and broader lever. If a source file is not matched by any mapping's `sources`, changes to it never trigger a check.

### Narrow your sources

If only your public packages have a documentation contract, map only those:

```toml
# Only public API packages are policed.
[[doc_mapping]]
sources = ["src/public/**", "pkg/api/**/*.go"]
docs = ["docs/reference/"]
```

Changes under, say, `src/internal/**` or `test/**` will not be checked at all, because they match no `sources` glob.

### Use `**` deliberately

- `internal/**` matches every file under `internal/` at any depth.
- `src/**/*.go` matches only `.go` files under `src/` at any depth.
- `*` stays within a single path segment; use `**` to cross directories.

Prefer the most specific glob that still covers your public surface — this reduces LLM calls and noise.

### Map different trees to different docs

Multiple `[[doc_mapping]]` entries let each area of code point at the docs that actually describe it, so a change only checks the relevant docs:

```toml
[[doc_mapping]]
sources = ["cmd/**"]
docs = ["docs/cli.md"]

[[doc_mapping]]
sources = ["internal/api/**/*.go"]
docs = ["docs/api/"]          # directory → every *.md inside
```

### Combine both tools

Scope broadly with `doc_mapping`, then use `driftlock:ignore` for the handful of exceptions inside an otherwise-tracked file. Together they let you enforce "docs must move with the public API" precisely, without false alarms.

See also: [Configuration → `[[doc_mapping]]`](./configuration.md) · [Architecture → parser](./architecture.md) · [Providers → economics](./providers.md).
