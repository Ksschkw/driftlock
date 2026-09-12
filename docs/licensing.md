# Licensing

**Decided: Business Source License 1.1, with `Change Date: 2030-06-01` and a
change licence of MIT.**

## What the licence allows

You may use, modify, and redistribute Driftlock for any purpose, **including in
production**. An individual developer acting on their own behalf, or an
organisation using it for its own internal purposes, may always do so without
paying anything.

## What the licence reserves

The one restricted use is offering Driftlock — or a derivative work that
directly competes with it — as a standalone hosted service (SaaS). That is the
commercial surface the licence protects: nobody can take this work and sell it
as a service without the author earning from it.

## It becomes open source

On **2030-06-01** this version automatically converts to **MIT**, with no
further conditions. That date is a real, near-term date, not a placeholder.

## Why not a permissive licence

An earlier revision of this repository moved the project to Apache-2.0 under an
"open core" model. It was reverted, for a concrete reason: Apache-2.0 grants
everyone the right to use, modify, distribute, sublicense, and **sell** the
software commercially, with no obligation to pay the author or share revenue.
Anyone could have sold Driftlock or run a paid hosted version of it and the
author would have earned nothing. Permissive licensing buys adoption by giving
up exactly the exclusivity that was wanted here.

BUSL-1.1 keeps the same practical adoption story for real users — every internal
and production use is permitted — while reserving the hosted-service business
for the author.

## Why 2030-06-01

BUSL 1.1 requires the change date to be no more than four years from the first
public distribution of the Licensed Work, and the licence text itself says the
change takes effect on the change date *or* the fourth anniversary of a
version's first public distribution, **whichever comes first**.

- The first public distribution was `v0.1.0` on 2026-06-02.
- Four years later is 2030-06-02.
- The change date is therefore set to **2030-06-01**, one day inside that
  boundary, so it is unambiguously within the permitted window.

The previous value was `2099-12-31`, which was off-spec and read to a reviewer
as "never becomes open source".

## What the licence does not do

- It does not grant trademark rights; you may not present a fork as the official
  Driftlock.
- It does not require anyone to publish modifications.
- It does not restrict internal or production use, which is what most users
  actually do.

## Relicensing

The project currently has a single copyright holder (every commit is by
`kookafor893@gmail.com`), so the licence can still be changed cleanly. Once an
outside contribution lands, that changes: relicensing would require either a
contributor licence agreement or explicit permission from each contributor. If
relicensing flexibility matters, put a CLA or DCO in place **before** accepting
outside patches.

## Not legal advice

This page records an engineering decision and its reasoning. It is not legal
advice; have a lawyer review the licence before relying on it commercially.
