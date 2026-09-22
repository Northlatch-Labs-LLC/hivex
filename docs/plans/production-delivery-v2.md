# Production Delivery Plan v2 — Hivex full suite (remade 2026-09-21)

Supersedes the 2026-09-19 full-stack plan (Phases 1–3 shipped and live-verified:
rebrand, portal/commerce, SDK/MCP; see `git log` 2026-09-20). One product, three layers:

1. **Harness** — Go agent core, this repo (`cmd/hivebot`, `internal/*`), office UI `web/`.
2. **Gateway (HiveAPI)** — embedded repo `9router/` (Next.js 16, live at :18080; portal + operator dashboard + OpenAI-compat API).
3. **Customer-facing web app** — the front door; today only the demo landing at `9router/src/app/landing/`.

## Carried-over state (mid-flight from previous session — verified green 2026-09-21)

Uncommitted working tree, all coherent and test-green:
- `identity.go` + tests: final residue purge of the retired pre-rebrand device-family alias (alias removed; test pin updated to HIVEX).
- Slack test fixtures: personal-name purge (fictional persona now "Ada Okafor").
- `docs/specs/*`: author → Northlatch Labs.
- `9router` gitlink at f5fd41f (gateway admin consolidation) — needs pointer commit.
- `apps/9router`: stale old checkout of the same repo (a8c9d38 is ancestor of f5fd41f) — remove.
- `hivex-home/.hivex/wiki`: organic office runtime edits — commit inside that repo.
- Team suite: one FAIL under heavy parallel load, clean on rerun — watch for flake.

## Slices (agile: each = build → gate → commit; one build agent at a time)

**S0 Reconcile the tree.** Commit the carried-over edits; remove `apps/9router` gitlink + dir; commit wiki runtime edits; record baseline (build/vet/lint/tests).
Gate: `go build ./...`, `go vet ./...`, `go test ./internal/openclaw/ ./internal/team/`, residue greps (retired brand name | old personal name) = 0.

**S1 Customer-facing web app (replaces the demo).** Rebuild `9router/src/app/landing/` into the real Hive storefront in the Hivex idiom (terminal-shell tokens from `164a1cd`: JetBrains Mono + Inter, amber #FFD44F/#FF9330 on charcoal #0A0F18, dot-grid, micro-labels): honest product story (Harness / Gateway / Portal), live pricing (Free / $49 Monthly / Business contact — verbatim strings only), CTAs wired to real routes (`/portal/signup`, `/portal/pricing`), SEO + mobile.
Gate: `npm run lint` + `npx vitest run` in 9router, build green, dead-link check = 0.

**S2 Gateway redesign in the Hivex idiom.** Complete the terminal-shell pass across operator dashboard surfaces and portal pages (consolidated theme tokens, mono micro-labels, status LEDs, WCAG AA), keeping function untouched.
Gate: 9router lint + baseline tests green; portal regression (95/95) green.

**S3 Harness hardening + residue.** Replace onboarding media that still shows the pre-rebrand name in pixels (`web/public/media/onboarding/` — regenerate brand-correct or clean stills); code-quality audit per `docs/CODE-QUALITY.md` on touched modules; office evals green.
Gate: `go test` critical packages, office evals, `bunx tsc --noEmit` + `bun run build` in `web/`.

**S4 Final production matrix.** Full gate across all layers + residue greps + delivery report (what shipped, what is user-provisioned: Stripe `whsec_…`, hosting token).

## Non-goals / user-provisioned
Stripe webhook secret; hosting-panel token for the :8000 panel; production DNS. Financial strings verbatim only. Docs and code stay MIT © 2026 Northlatch Labs LLC.
