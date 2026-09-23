# Gridframe Systems — Agile Operations SOP (v1.0)
**Mandatory operating procedure for all agents and departments**
**Effective:** 2026-09-22 (Sprint S-2026-W39) · **Owner:** EXE-00 Atlas · **Approver:** HOB-00 Principal
**Prime directive:** Grow revenue while maximizing **Aggregate Efficiency of resources (AEI)**. Efficiency is not a report; it is a constraint on every sprint, every approval, and every reappraisal.

---

## 1. Operating Principles (non-negotiable)

1. **Aggregate efficiency is mandatory.** No work enters a sprint unless it links to a KPI and its cost line. AEI is computed monthly, published unedited, and drives reappraisal + rebalancing.
2. **Everything is a record.** Decisions, money, exceptions, and approvals live in the ledgers (doc 07). Unrecorded = invalid.
3. **Human-on-the-loop.** Agents execute; the Principal approves what tiers require (see §5). The human-only reserve list is absolute.
4. **Sprint or it does not happen.** All departments work on the same weekly sprint grid.
5. **Escalate fast, fail small.** Exceptions follow §6 severity model. A 15-minute escalation beats a 2-day silent grind.
6. **Written beats verbal.** All inter-agent handoffs use the interface contracts (doc 02 §6).

---

## 2. AEI — Aggregate Efficiency Index (the company's prime metric)

**AEI = Value Created ÷ Resources Consumed**, computed monthly by OPS-01 Tally, verified by RSK-03 Audit, published on day 3.

| Component | Definition | Source |
|---|---|---|
| **V** (value) | V = R + 0.10·P, where R = cash-basis recognized revenue; P = net-new qualified pipeline (deal value, verified by Vector) | revenue-ledger.csv, CRM records |
| **C** (cost) | C = D + L + A, where D = direct COGS (cloud compute, third-party inference for the product, vendor tooling); L = human labor (supervision hours × $85/h loaded rate); A = internal agent inference cost (Z.AI tokens attributed to company operations) | cost-ledger.csv, Z.AI usage reports |

**Thresholds and forced responses:**

| AEI (month) | Status | Mandatory response |
|---|---|---|
| < 1.0 | 🔴 Survival | Freeze T1+ discretionary spend; Atlas presents recovery plan to Principal within 48h |
| 1.0 – 2.0 | 🟠 Warning | Department leads submit efficiency actions in next sprint; Cipher runs cost-per-activation review |
| 2.0 – 4.0 | 🟢 Healthy | Continue; reinvest margin per plan |
| > 4.0 | 🔵 Investigate | Likely underinvestment (starving growth). PA + Vector propose demand-side spend increase |

**Targets:** AEI ≥ 1.5 by M3, ≥ 2.0 by M6, **≥ 3.0 by M12** (with revenue growth ≥ base-case plan — see Revenue & Pricing Model, doc 06). Gross margin ≥ 75% by M6. Department-level efficiency (AEI-d) uses the same formula with department-attributable V and C; attribution rules owned by PPL-01 Gauge.

**Anti-gaming:** Audit samples ≥ 5% of value/cost entries quarterly; pipeline counted only when deal-value verified; revenue is cash-basis (booked when invoiced/paid, never "arranged").

---

## 3. Cadence (all times Pacific)

| Ceremony | When | Chair | Participants | Output |
|---|---|---|---|---|
| **Day plan** | Daily 08:00 | Meridian | All leads | Priorities + queue preview in #day-plan |
| **Standups (async)** | Daily by 09:00 | Dept leads | All agents | 3-liner: yesterday / today / blockers |
| **Principal digest** | Daily 17:30 | Meridian | Principal | KPIs, exceptions, T2+ approval queue (≤ 10 items) |
| **Sprint planning** | Mon 10:00 | Atlas | Leads + owners | Sprint goal + committed backlog |
| **Release train** | Thu | Forge | PE + OPS | Tagged release, notes to RG |
| **Sprint review + retro** | Fri 15:00 | Atlas | All | Demo notes; retro actions ≤ 3, owners named |
| **Monthly close** | Day 1–3 | Tally | OF + RL | Ledgers closed; AEI report; MBR pack |
| **Monthly Business Review (MBR)** | Day 4 | Atlas | Leads + Principal | Scorecards vs plan; AEI actions; next-month targets |
| **Quarterly QBR + Reappraisal** | Week 1 of quarter | Principal | All | Agent scorecards; rebalancing; policy review; SOP amendments |

Sprint grid: 1-week sprints, Monday–Friday. A sprint without a goal is invalid; a goal without a KPI link is invalid.

---

## 4. Sprint Framework

- **Sprint goal:** single sentence, KPI-linked, set by Atlas with the owning lead.
- **Capacity:** planned in cost-weighted agent-days (tier A day ≈ 3× tier C day for planning purposes). Reserve 20% for exceptions/support.
- **Work item states:** `proposed → ready → in-progress → review → done → ledgered`.
- **Definition of Ready (DoR):** owner agent named; acceptance criteria testable; size ≤ 2 agent-days (else split); KPI + cost line linked; dependencies flagged.
- **Definition of Done (DoD):** meets acceptance criteria; second-agent review passed (peer tier ≥ B); docs updated (Tessellate for external surfaces); ledger entry written where money/records involved; release notes if customer-visible.
- **WIP limit:** 2 in-progress items per agent. Blocked > 4h → escalate to lead; > 1 day → Meridian reprioritizes.
- **Across departments:** every department commits ≥ 1 item to each sprint (Sales runs experiments as sprint items; Ops runs SLA workstreams; RL runs control checks). One grid, one company.

**Example sprint (S-2026-W39, current):** Goal: "First paying self-serve customers onboarded with zero SEV1/SEV2 and signup→env-boot p95 < 90s." Items span PE (runtime polish), RG (launch offers), OF (onboarding checklist), RL (records retention policy), PA (scorecard v1 seed).

---

## 5. Approvals & Autonomy (operating rules)

Use the Autonomy Ladder and Approval Tiers from doc 02 §1. Operating rules:

1. Every T1+ action creates a row in `approval-queue.csv` **before** execution (pre-approval), except SEV1 emergencies (post-log within 15 min).
2. T2 items batch into the daily 17:30 digest; Principal decision recorded same day where possible; anything undecided > 48h auto-escalates to the next MBR with a "cost of delay" estimate attached.
3. T3 always pairs Counsel (RSK-01) prep + Principal signature. No exceptions, including "urgent" customer asks — Harbor knows to trade scope, not governance.
4. Agents may always **propose** (L0) — proposals are cheap; unauthorized execution is an SEV3 exception at minimum, SEV2 if external.
5. Monthly, Audit samples approval compliance; violations feed the responsible lead's scorecard.

---

## 6. Exception Handling (incidents + anomalies)

**Severity model:**

| Sev | Definition | Response | Notify Principal | Escalation |
|---|---|---|---|---|
| **SEV1** | Production down, security breach, data loss, revenue blocked | On-call (Bastion) acts immediately under L3; war room; updates every 15 min | ≤ 30 min | Auto-escalate at 60 min unresolved |
| **SEV2** | Degraded, customer-impacting, financial anomaly > $250 | Dept lead engages; 4h target | Daily digest | 8h → Atlas |
| **SEV3** | Minor/internal, process deviation | Ticketed; next business day | Weekly | 3 days → lead review |

**Standing rules:**
- Anyone (any agent) may declare an exception; declaring is never punished, hiding is.
- Financial anomaly > $250 unexplained → freeze affected non-essential spend (Balance) + Audit investigation within 24h.
- Security anything → Warden + Aegis immediately; Counsel assesses notification duties.
- Every SEV1/SEV2 gets a postmortem within 48h (fields: timeline, impact, cause, cost, action items with owners and dates) filed to the records; retro actions enter next sprint — mandatory.

---

## 7. Records, Retention & Data Persistence

- Ledgers live in `DELIVERY/ledgers/` (structure in doc 07) — the dashboard (doc 08) writes the same schema via its local store; CSV exports drop directly into the folder.
- Financial records: retain 7 years. Decision/exception logs: 3 years. Sprint boards: 1 year, then archive.
- Monthly close locks the month's CSVs (append-only afterwards; corrections as new reversing rows, never edits — Tally enforces).
- Back-office trail requirement: every customer order → `revenue-ledger.csv` row + invoice reference; every approval → `approval-queue.csv` row; every incident → `exception-log.csv` row. The dashboard's live register and the CSVs are one system (same schema, same IDs).

---

## 8. Onboarding a New Agent (runbook)

1. Scout drafts role card (mission, KPI, autonomy, tier) → T2 approval.
2. Mentor builds playbook pack: SOP, relevant department charter, interface contracts, templates.
3. Chorus registers agent in roster doc + scorecard sheet; Gauge adds cost line.
4. 2-sprint shakedown at L1 with 100% output sampling; Audit reviews; then full autonomy level applies.

---

## 9. Compliance Calendar (owned by RSK-01 Counsel)

Maintained in the registration playbook (doc 04) with state-specific deadlines; Counsel verifies monthly; a missed statutory deadline is an automatic SEV2 with root-cause to the Principal.

---

## 10. Meeting Hygiene

Async first. No synchronous meeting exceeds 45 min (QBR exempt). Every ceremony produces a written artifact the same day; agents who cannot attend consume the artifact. Meridian keeps a running "decision debt" list — questions awaiting the Principal — and caps it at 10.

---

## 11. Change Control

This SOP is versioned. Amendments: any agent may propose (L0 memo to Meridian) → Atlas reviews → Principal approves (T2) → version bump + changelog line + Mentor briefs all agents within 2 business days. Emergency amendments (legal/incident-driven) may apply immediately with Counsel co-sign and retroactive Principal ratification at the next digest.

---

## 12. Quick Reference Card (post at every workstation)

- **Morning:** read day plan → confirm your sprint items → blockers up by 09:00.
- **Spend/contract/legal?** Check tier. T1 lead-approve & log · T2 queue to Principal · T3 Counsel + Principal. **When unsure, escalate one tier.**
- **Something broke?** Declare it. SEV1 act now (L3), tell everyone ≤ 30 min. SEV2 4h. SEV3 tomorrow.
- **Month end:** close by day 3 → AEI published → MBR day 4 → actions into sprint.
- **Quarter:** scorecards → reappraisal → rebalance capacity to marginal AEI.
- **AEI below 2.0?** You have efficiency actions due in the next sprint. Above 4.0? Expect growth-investment proposals.
