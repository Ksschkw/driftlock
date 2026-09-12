# Releasing

Releases are built by [`.github/workflows/release.yml`](../.github/workflows/release.yml)
whenever a tag matching `v*` is pushed. Nothing is built by hand.

## What a release produces

For each platform, a binary plus a per-asset checksum:

```
driftlock-linux-amd64
driftlock-linux-amd64.sha256
driftlock-linux-arm64
driftlock-linux-arm64.sha256
driftlock-darwin-amd64
driftlock-darwin-amd64.sha256
driftlock-darwin-arm64
driftlock-darwin-arm64.sha256
driftlock-windows-amd64.exe
driftlock-windows-amd64.exe.sha256
checksums.txt          # all binaries, one line each
```

The workflow verifies its own `checksums.txt` before publishing, so a truncated
or corrupted upload cannot ship.

## Version metadata

Each binary is built with the release metadata injected:

```
go build -trimpath -ldflags "\
  -X main.version=$TAG \
  -X main.commit=$(git rev-parse --short HEAD) \
  -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -s -w" -o dist/driftlock-linux-amd64 ./cmd/driftlock
```

A binary built any other way reports `driftlock version` as `dev`, which makes
it obvious that a bug report is against unreleased code.

`-trimpath` and `CGO_ENABLED=0` keep builds reproducible and static.

## Cutting a release

```bash
git tag -a v0.4.0 -m "v0.4.0"
git push origin v0.4.0
```

Then confirm on the release page that every asset and both checksum forms are
present, and that the smoke workflow is green on `main`.

## Checksum verification

Both installers verify the downloaded binary:

1. `<asset>.sha256` if it exists;
2. otherwise the asset's line in `checksums.txt`.

If a checksum is available and does **not** match, the installer aborts. A
missing checksum produces a warning and continues, since that only happens for
an old release that predates checksum publishing.

## Keeping the source tree clean

Build output goes to `dist/`, which is gitignored. Do not build release
binaries into the repository root; the earlier workflow did, which left ~75 MB
of platform binaries in the working tree.
