# Licensing

**Decided: the project is Apache-2.0 with an open-core model.**

The CLI and libraries in this repository are licensed under the Apache License,
Version 2.0 (see [LICENSE](../LICENSE)). You may use, modify, self-host, and
redistribute them freely, including commercially and inside an organisation, and
you may build products on top of them.

Optional commercial offerings operated by the copyright holder — for example a
hosted service or a team management surface — are separate products under
separate terms. They are not required to use, build, or self-host anything here.
[NOTICE](../NOTICE) records the attribution and that boundary.

## Why

Driftlock is a CLI that a team installs on every developer machine and in CI,
and that runs inside a git hook. Tools in that position appear in a
legal/security review before they appear in a build, so permissive licensing is
doing real product work: it removes the review friction that would otherwise
stop the tool being adopted at all.

## What changed, and why

The project previously shipped the Business Source License 1.1 with
`Change Date: 2099-12-31`, which had three concrete defects:

1. **The change date was off-spec.** BUSL 1.1 caps the change date at four years
   from a version's first public distribution — the licence text itself said the
   fourth anniversary applies "whichever comes first" — so the stated date did
   not do what it appeared to do, while reading to a reviewer as "never becomes
   open source".
2. **The README and the grant disagreed.** The README described use as free for
   "any non-commercial purpose", which did not match the broader Additional Use
   Grant.
3. **It was a promise no reviewer would rely on**, which is the opposite of what
   a licence is for.

## Options considered

| Option | Outcome |
| --- | --- |
| **Apache-2.0 (chosen, as open core)** | Maximum adoption for the core CLI; explicit patent grant; monetisation moves to optional hosted/team offerings. |
| MIT | Equivalent adoption, no explicit patent grant. |
| BUSL-1.1 with a correct 4-year change date | Source-available; still blocked by some corporate policies. |
| Keep BUSL-1.1 as it was | Rejected: off-spec date plus ambiguous README. |

## Notes for contributors

Contributions are accepted under the Apache License, Version 2.0 (section 5 of
the licence: a contribution intentionally submitted for inclusion is under the
same terms unless explicitly stated otherwise). No separate CLA is in place.
