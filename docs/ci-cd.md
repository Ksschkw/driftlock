# CI/CD integration

Driftlock is designed to run in two places: locally via the [pre-commit hook](./getting-started.md) and in CI against pull requests. In CI it runs in **range mode** — comparing two Git refs — and it **never modifies files**; it only passes or fails. This page covers the GitHub Action, the [`pre-commit`](https://pre-commit.com) framework, and running Driftlock in any CI system.

## The command CI runs

Regardless of platform, CI runs the read-only check between the PR base and head:

```bash
driftlock check --base <base-ref> --head <head-ref>
```

- Exits **non-zero** when documentation is out of sync — this is what fails the job.
- Add `--report` to always exit `0` (report drift without failing) — for gradual rollout.
- Add `--json` to emit a machine-readable report to stdout.

Because it compares two commits, **both refs must exist in the CI checkout.** With shallow clones they do not — see the `fetch-depth: 0` gotcha below.

---

## GitHub Actions

Driftlock ships a composite Action at the repository root (`action.yml`), used as `Ksschkw/driftlock@main`.

### Full example workflow

An example lives in the repo at `.github/workflows/driftlock.yml`. A complete PR-gating workflow:

```yaml
name: Driftlock

on:
  pull_request:

jobs:
  docs-drift:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0          # REQUIRED — see gotcha below

      - name: Driftlock
        uses: Ksschkw/driftlock@main
        with:
          api-key: ${{ secrets.DRIFTLOCK_API_KEY }}
          # base and head default to the PR base/head; usually leave them out.
          # report-only: 'true'   # uncomment during rollout
```

### Action inputs

| Input | Default | Description |
| --- | --- | --- |
| `base` | `${{ github.event.pull_request.base.sha }}` | Base git ref to compare against (the PR target). |
| `head` | `${{ github.sha }}` | Head git ref (the PR tip). |
| `version` | `latest` | Driftlock version to install — a git tag, or `latest`. Installed via `go install ...@<version>`. |
| `api-key` | `''` | LLM API key. Exposed to Driftlock as `$DRIFTLOCK_API_KEY`. **Pass a repository secret.** |
| `report-only` | `'false'` | When `'true'`, report drift without failing the job (adds `--report`, exit 0). |
| `working-directory` | `'.'` | Directory containing `.driftlock.toml`. |

> The Action sets up Go 1.24, installs Driftlock with `go install github.com/Ksschkw/driftlock/cmd/driftlock@<version>`, then runs `driftlock check --base <base> --head <head>` (adding `--report` when `report-only` is `'true'`).

### The `fetch-depth: 0` gotcha

`actions/checkout` performs a **shallow clone by default** (only the tip commit). Driftlock's range check needs *both* the base and head commits present locally, so it can parse the old and new content of changed files. Without the full history the base ref is missing and the check fails with an error like "could not resolve base ref".

**Always set `fetch-depth: 0`** on the checkout step:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

This is the single most common CI setup mistake — see [Troubleshooting](./troubleshooting.md).

### Storing the API key secret

1. In your repository, go to **Settings → Secrets and variables → Actions → New repository secret**.
2. Name it `DRIFTLOCK_API_KEY` and paste your provider key (see [Providers](./providers.md)).
3. Reference it in the workflow: `api-key: ${{ secrets.DRIFTLOCK_API_KEY }}`.

The Action exposes it to Driftlock as the environment variable `$DRIFTLOCK_API_KEY`, which your `.driftlock.toml` reads via `api_key = "${DRIFTLOCK_API_KEY}"`. Never commit a literal key.

### Report-only rollout strategy

Turning Driftlock on as a **required** check on a repo whose docs are already out of sync will make every PR red on day one. Roll out in three phases:

1. **Observe.** Set `report-only: 'true'`. The job always passes but prints exactly which docs have drifted. Use this to see the size of the problem.
2. **Backfill.** Bring existing docs into sync (locally, `driftlock fix` regenerates mapped docs for staged files). Keep report-only on until PRs are consistently clean.
3. **Enforce.** Remove `report-only` (or set it to `'false'`) and mark the Driftlock job as a **required status check** in branch protection. From now on, drifted PRs are blocked from merging.

---

## The `pre-commit` framework

If your team standardizes local hooks with the [`pre-commit`](https://pre-commit.com) framework, Driftlock provides a hook definition. The repo includes `.pre-commit-hooks.yaml` exposing a hook with id **`driftlock`**.

Add it to your project's `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/Ksschkw/driftlock
    rev: main            # or a tag
    hooks:
      - id: driftlock
```

Then install the hooks:

```bash
pre-commit install
```

Notes:

- The hook runs `driftlock hook-run` (`language: system`), so the **`driftlock` binary must be on your `PATH`** — the `pre-commit` framework does not install it for you. Install it per [Getting started](./getting-started.md).
- It is configured with `always_run: true` and `pass_filenames: false`; Driftlock inspects the staged index itself rather than receiving a file list.
- This replaces the raw `.git/hooks/pre-commit` script that `driftlock init` installs — use one mechanism or the other, not both.

---

## Running in any CI system

Driftlock has no GitHub-specific dependency. In any runner:

1. **Ensure full history** (fetch both refs — the equivalent of `fetch-depth: 0`).
2. **Install** the binary (`go install github.com/Ksschkw/driftlock/cmd/driftlock@latest`, or use a prebuilt binary).
3. **Export** `DRIFTLOCK_API_KEY` from your CI secret store.
4. **Run** the range check with JSON output if you want to post-process it.

### GitLab CI example

```yaml
driftlock:
  image: golang:1.24
  variables:
    GIT_DEPTH: 0                 # full history, so both refs exist
  script:
    - go install github.com/Ksschkw/driftlock/cmd/driftlock@latest
    - export PATH="$PATH:$(go env GOPATH)/bin"
    - driftlock check
        --base "origin/$CI_MERGE_REQUEST_TARGET_BRANCH_NAME"
        --head "$CI_COMMIT_SHA"
        --json
```

### Generic shell (any runner)

```bash
export DRIFTLOCK_API_KEY="$CI_SECRET_LLM_KEY"

driftlock check --base "$BASE_SHA" --head "$HEAD_SHA" --json | tee drift-report.json

# The exit code drives the pipeline: non-zero = drift = fail.
```

Use `--report` if you want the pipeline to stay green while still capturing the JSON report as an artifact during rollout.

See also: [Configuration](./configuration.md) · [Providers](./providers.md) · [Troubleshooting](./troubleshooting.md).
