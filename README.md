# Driftlock

Driftlock is a commit-time gatekeeper that detects when your documentation has
fallen behind your code—and then fixes it for you.

It watches every `git commit`, compares the actual structural changes (function
signatures, types, classes) with the documentation you claim describes them,
and if there’s a mismatch, it blocks the commit, rewrites the affected
documentation, and tells you to stage the new version. No more “I’ll update the
docs later.”

Optionally, it can log an immutable audit trail to a Solana devnet contract,
because some of you work in industries where proving that docs matched code at
every commit is a regulatory requirement.

## Installation

### With Go

If you have Go 1.22+:

```
go install github.com/Ksschkw/driftlock/cmd/driftlock@latest
```

### Pre-built binaries

Download a static binary from the [Releases page](https://github.com/Ksschkw/driftlock/releases).
Linux, macOS, and Windows builds are available. Place the binary somewhere in
your `PATH`.

### Shell installer

```
curl -fsSL https://raw.githubusercontent.com/Ksschkw/driftlock/main/install.sh | sh
```

Yes, that pipes a shell script into your shell. It downloads the right binary
for your OS and architecture, verifies a checksum, and drops it into
`/usr/local/bin`. If that terrifies you (it should), you can inspect the script
first at the same URL.

## Quick start

```bash
cd your-project
driftlock init              # sets up the hook and a .driftlock.toml
git add . && git commit -m "commit message"
# If your docs are out of sync, the commit is blocked and the docs are updated.
```

## Why

Because out-of-date documentation is technical debt that compiles. Driftlock
treats it as a build failure.

## License

MIT