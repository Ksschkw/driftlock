# Driftlock Documentation

Driftlock is a commit-time and CI gatekeeper that blocks changes when a **structural code change** — an added, removed, or modified public function, method, type, class, interface, or struct signature — is not reflected in the Markdown documentation you have mapped to that code. When drift is detected, Driftlock can auto-rewrite the affected documentation with an LLM and block the commit so you can review the result before it lands. An optional Solana audit trail records a tamper-evident hash of every verdict.

Driftlock parses source in **any language** through a per-language regex extractor that strips comments and string literals *before* matching, so it never false-triggers on tokens buried in a comment or a string. It only runs the patterns relevant to each file's extension, and its verdicts are content-addressed and cached so identical checks never re-hit (or re-bill) the LLM.

## Who is this for

Teams that treat documentation as part of the contract — API reference docs, SDK guides, public README surfaces — and are tired of docs silently rotting behind the code. Driftlock enforces the "docs must move with the API" rule automatically, in two places:

- **Locally**, through a Git `pre-commit` hook, so drift is caught before it is ever committed.
- **In CI**, through a GitHub Action (or any CI runner) that fails a pull request whose docs have drifted.

Adopt it gradually with report-only mode, then flip it to blocking once your docs are in sync.

## Table of contents

| Guide | What it covers |
| --- | --- |
| [Getting started](./getting-started.md) | Install Driftlock, run `driftlock init`, set your API key, and see your first blocked commit. |
| [Tutorial](./tutorial.md) | A long, hands-on walkthrough: build a sample repo, watch a commit get blocked, review an auto-fix, use `driftlock:ignore`, and wire up CI. **Start here if you learn by doing.** |
| [Configuration](./configuration.md) | Exhaustive reference for every `.driftlock.toml` field, environment variables, and prompt overrides. |
| [CI/CD](./ci-cd.md) | GitHub Actions, the pre-commit framework, and running Driftlock in any CI system. |
| [Providers](./providers.md) | LLM provider setup for OpenRouter, Groq, DeepSeek, Together, vLLM, and Ollama, plus cost economics. |
| [Ignoring symbols](./ignoring.md) | The `driftlock:ignore` annotation and scoping `doc_mapping` to avoid over-triggering. |
| [Caching](./caching.md) | How the content-addressed verdict cache works, where it lives, and when it invalidates. |
| [Architecture](./architecture.md) | The end-to-end pipeline, package layout, and key design invariants. |
| [Troubleshooting](./troubleshooting.md) | Fixes for blocked commits, LLM errors, parser surprises, CI ref problems, and cache staleness. |

## Command summary

| Command | Purpose |
| --- | --- |
| `driftlock init` | Interactive guided setup: writes `.driftlock.toml`, installs the pre-commit hook, updates `.gitignore`. |
| `driftlock check [--base REF] [--head REF] [--report] [--json]` | Run the checks without ever writing files. Local staged mode by default; CI range mode with `--base`. |
| `driftlock fix` | Force-regenerate all mapped documentation for staged files. |
| `driftlock hook-run [--no-fix]` | Internal command invoked by the pre-commit hook. |
| `driftlock log` | Show the last 20 audit-log entries. |
| `driftlock status` | Show current status. |

See [Getting started](./getting-started.md) to install.
