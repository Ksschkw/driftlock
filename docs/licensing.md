# Licensing: a decision brief

**Status: awaiting a maintainer decision. Nothing has been changed.**

The `LICENSE` file is Business Source License 1.1 with these parameters:

| Parameter | Value |
| --- | --- |
| Licensor | Okafor Kosisochukwu Johpaul |
| Licensed Work | Driftlock |
| Additional Use Grant | Any purpose including production, except offering the Licensed Work as a standalone hosted service (SaaS) or a derivative that directly competes with the official service |
| Change Date | **2099-12-31** |
| Change License | MIT |

## Why this matters for this particular product

Driftlock is not a library a developer imports; it is a CLI that a team must
install on every developer machine and in CI, and it runs inside a git hook. That
means:

- it appears in a legal/security review before it appears in a build;
- every engineer who clones the repository is expected to install it;
- CI configuration referencing it is committed to the repository.

For tools in that position, source-available restrictions are a much larger
adoption tax than they are for a server-side dependency. A permissive license is
therefore doing real product work, not just being generous.

## Problems with the current parameters

1. **The change date is off-spec.** BUSL 1.1 caps the change date at four years
   from the first public distribution of each version; the license text in this
   repository says exactly that ("Effective on the Change Date, or the fourth
   anniversary of the first publicly available distribution of a specific
   version of the Licensed Work under this License, whichever comes first").
   A `2099-12-31` change date therefore does not do what it appears to do — the
   fourth-anniversary clause governs — while reading to a reviewer as "never
   becomes open source". That is the worst of both worlds.

2. **The README and the grant disagree.** The README says use is free "for any
   non-commercial purpose, including personal use and internal use within an
   organisation". The Additional Use Grant here is broader than that (it permits
   production use, not just non-commercial use) but is buried under BUSL's
   default "non-production use" language. A reader cannot tell which applies.

3. **"Automatically become MIT on 2099-12-31"** is stated in the README as a
   feature. It is not one; it is a promise no reviewer will rely on.

## Options

| Option | What it means | Cost |
| --- | --- | --- |
| **A. MIT or Apache-2.0** | Maximum adoption. Anyone can use, modify, and embed it, including in commercial products. | No licensing leverage if a paid product is planned. |
| **B. BUSL-1.1 with a 4-year change date** | Source-available; converts to MIT/Apache four years after each release. | Still blocked by some corporate policies, and needs careful per-version dating. |
| **C. Open core** | Permissive core (MIT/Apache), commercial offering around it (hosted service, team dashboard, policy management). | Requires a commercial product to actually exist to be meaningful. |
| **D. Keep as-is** | No change. | Off-spec change date plus ambiguous README; not recommended. |

## Recommendation

For a commit-hook CLI whose value depends on being installed everywhere, **A** or
**C**. If monetisation is planned, **C** gives both: the hook itself stays
permissive, which is what drives adoption, and the paid surface is the hosted
service rather than the binary.

If **D** is chosen deliberately, at minimum fix the change date to a real date
within four years and align the README wording with the Additional Use Grant.

## What has been done here

Nothing in `LICENSE` or `README.md` has been modified. This brief records the
issue, the options, and a recommendation so the decision can be made quickly and
in one place. `TASKS.md` tracks the item as blocked on this decision.
