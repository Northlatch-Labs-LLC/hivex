# Gridframe Systems — Business Architecture (v1.0)
**Date:** 2026-09-22 · **Owner:** EXE-00 Atlas · **Approver:** HOB-00 Principal

> Working name "Gridframe Systems" (Assumption A-1). Product: the **Gridframe Suite** — an infrastructure development environment platform, already built, versioned, and deployed in production (Assumption A-3).

---

## 1. Business Definition

**What we sell:** Managed infrastructure development environments and agent runtimes — sandboxed, API-first environments where (a) AI agents execute and verify real work, (b) developers get reproducible cloud dev environments, and (c) companies operate fleets of environments under one control plane.

**Who buys:** Three segments —
1. **AI agents & their operators** (agent-first APIs: spawn, execute, verify, bill per use)
2. **Developers & teams** (cloud dev environments, instant boot, reproducible workspaces)
3. **Companies** (managed fleets, governance, compliance, private deployment)

**How we win:** An AI-operated cost structure (Z.AI GLM inference doing the work of a 25–40 person company under one human supervisor) converts directly into **price leadership at high gross margin** — plus an execution cadence (weekly releases, daily audits) classic competitors can't match headcount-for-headcount. Differentiation detail vs exe.dev / boat.dev and adjacent players: see Market & Competition (doc 05).

---

## 2. Revenue Architecture

| # | Stream | Segment | Model | Owner | Notes |
|---|---|---|---|---|---|
| 1 | **Self-serve subscriptions** | Developers, small teams | SaaS tiers (Free / Team / Scale) + metered compute | Vector | Primary early engine; pricing in doc 06 |
| 2 | **Usage-based agent API** | AI-agent operators | Per environment-hour / per verified execution | Vector | Differentiated, agent-first billing |
| 3 | **Enterprise contracts** | Companies | Annual platform + support tiers | Harbor | Land-and-expand via compliance needs |
| 4 | **Managed private deployment** | Companies | Setup + retainer | Harbor | High ACV, Bolt-on after SOC 2 groundwork |

**Order-to-cash flow:** signup → self-serve checkout (Stripe) or enterprise order record (Harbor → OF) → provisioning (automated) → metering aggregation → invoice/receipt → `revenue-ledger.csv` + dashboard register. Every order produces a persistent, auditable record (SOP §7).

---

## 3. Cost Architecture (what AEI divides)

| Block | Contents | Managed by |
|---|---|---|
| **D — Direct COGS** | Cloud compute for customer environments, egress, third-party inference consumed by the product | Bastion (cost/env KPI) |
| **L — Human labor** | Principal supervision hours (loaded $85/h), any fractional contractors | Chorus tracks, Principal owns |
| **A — Agent inference** | Z.AI GLM tokens attributed to operating the company (all 25 agents) | Gauge attributes by agent |

**Cost doctrine:** D scales with revenue (target ≤ 25% of revenue by M6 → gross margin ≥ 75%); L is capped (supervision only); A is budgeted per agent monthly and is the primary reappraisal lever (tier right-sizing). See SOP §2 and doc 06.

---

## 4. Growth Engine

1. **Product-led base:** Free tier with generous-but-metered environments → activation (first env boot < 90s) → Team conversion.
2. **Agent-ecosystem pull:** public API + SDKs + usage examples tuned for agent operators; agent frameworks list us as an execution backend.
3. **Content engine (L1):** Echo publishes technical content weekly (all public drafts T1-approved; anything price/legal/public-statement goes T2+).
4. **Enterprise attach:** compliance-driven deals (governance, audit trails — our own ledgers are the demo).
5. **Efficiency flywheel:** AEI > target → margin reinvested into demand experiments → revenue grows faster than cost (the aggregate-efficiency mandate in one loop).

---

## 5. KPI Tree (company → department → agent)

| Level | Metric | Target (M12) | Owner |
|---|---|---|---|
| Company | AEI | ≥ 3.0 | Atlas |
| Company | MRR | Base-case per doc 06 | Vector |
| Company | Gross margin | ≥ 75% | Tally/Bastion |
| RG | New MRR / pipeline coverage 3× / CAC payback < 12 mo / churn < 3%/mo | Vector |
| PE | Uptime ≥ 99.9% · releases ≥ 2/wk · infra cost per active env ↓ QoQ | Forge |
| OF | First response < 4h · resolution < 24h · close by day 3 · ledger error < 0.5% | Balance |
| RL | Zero missed statutory deadlines · audit coverage ≥ 5% | Aegis |
| PA | 100% quarterly scorecards · cost per unit of work ↓ QoQ | Chorus |

Every agent's personal KPI rolls up into a department KPI (roster, doc 02 §4). Gauge publishes attribution rules; Audit verifies.

---

## 6. Automation Stack (how the company runs itself)

| Layer | Component | Implementation (current) |
|---|---|---|
| Reasoning | 25-agent roster, tiered Z.AI GLM (A/B/C) | Doc 02; prompts/policies versioned by Mentor |
| Execution surface | Agents operate: repo & CI/CD, hosting control plane, billing (Stripe), support desk, CRM records, analytics, security tooling | Access mapped by role; least privilege; Warden audits |
| Records | CSV ledgers + dashboard local store (one schema) | Doc 07 + doc 08; monthly close locks |
| Cadence engine | Day plans, digests, sprint boards | Meridian; artifacts filed to records |
| Approvals | Tiered queue with pre/post logging | SOP §5; approval-queue.csv |
| Audit | Output sampling ≥ 5%, ledger checks, exception reviews | RSK-03 |
| Human layer | Principal: approval queue, digests, QBR, veto | T2/T3 items daily |

**Stack principle:** every tool the agents use must produce a machine-readable trail ingestible by the ledgers. If a tool can't be audited, it isn't adopted (T2 to change).

---

## 7. Legal & Entity Architecture

- **Entity:** single US LLC initially (Delaware default — rationale and Wyoming comparison in doc 04). Owner = human Principal. The company is AI-**operated**, human-**owned**: agents are instruments, not legal agents; signature authority stays with the Principal (Aegis enforces, Counsel drafts).
- **Conversion trigger:** institutional fundraising or > $1M ARR → evaluate C-Corp conversion (path pre-documented in doc 04 to avoid re-architecture).
- **Compliance surface:** federal tax elections, state franchise/annual filings, sales-tax nexus watch (SaaS/infra rules by state), FinCEN BOI monitoring (status as of 2026: doc 04 verifies), SOC 2 Type I → II track for enterprise motion.
- **Records:** company records = the ledgers + docs in this package; Counsel's calendar is authoritative for statutory deadlines.

---

## 8. Risk Register (top 8, reviewed weekly by Aegis)

| # | Risk | L | I | Mitigation | Owner |
|---|---|---|---|---|---|
| R1 | Key-person: single human Principal unavailable | M | H | Playbooks + L3 emergency autonomy; documented decision thresholds | Aegis |
| R2 | Platform/infra outage hits uptime SLA | M | H | Multi-AZ, rollback drills, status page; SEV1 runbook | Bastion |
| R3 | Security breach of customer environments | L | H | Isolation architecture, secrets hygiene, incident response, cyber insurance | Warden |
| R4 | Compliance miss (filing/tax/BOI change) | L | H | Counsel calendar + monthly check; auto-SEV2 | Counsel |
| R5 | Price war from funded competitors | M | M | Cost-structure advantage; avoid bidding to zero; value anchoring | Vector |
| R6 | Model/provider dependency (Z.AI pricing/limits) | M | M | Budget caps; workload tiering; portability review quarterly | Forge/Gauge |
| R7 | Unmonitored agent error → external damage | M | M | Tier approvals, ≥ 5% audit sampling, human reserve list | Audit |
| R8 | Revenue concentration (one enterprise > 30%) | M | M | Watch trigger at 20%; diversification push | Vector |

(L = likelihood, I = impact. Full register maintained in the records; this table is the standing summary.)

---

## 9. 12-Month Milestone Arc (summary — detail in doc 06)

- **M0–M1:** entity live, bank + billing wired, self-serve launch, AEI baseline published.
- **M2–M3:** first paying cohort, AEI ≥ 1.5, first reappraisal cycle.
- **M4–M6:** agent-API GA, AEI ≥ 2.0, gross margin ≥ 75%, first enterprise deal in motion.
- **M7–M9:** enterprise close(s), SOC 2 Type I readiness, AEI ≥ 2.5.
- **M10–M12:** AEI ≥ 3.0, base-case MRR, Q4 full audit + year-1 tax package clean.

---

## 10. Assumptions

A-1 placeholder name · A-2 Delaware LLC default · A-3 suite already in production · A-4 USD / Pacific · A-5 registration, banking, tax, contracts, public price changes are human-reserve actions. (Full text in doc 02 §7.)
