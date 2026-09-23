# Gridframe Systems — Organization & Agent Roster
**Version:** 1.0 · **Date:** 2026-09-22 · **Owner:** Human Principal (HOB-00) · **Status:** Ratified pending Principal countersignature
**Next reappraisal:** 2026 Q4 review cycle

> **Naming note:** "Gridframe Systems" is a working name (Assumption A-1). Verify availability during entity registration (see the Bizee Registration Playbook) before any public use.

---

## 1. Governance Model — Human-on-the-Loop

The company is operated by AI agents (Z.AI GLM inference models) under one human authority: the **Human Principal**. The Principal does not manage tasks; the Principal owns outcomes, approves reserved actions, and audits the machine.

**Autonomy Ladder (applies to every agent):**

| Level | Meaning | Examples |
|---|---|---|
| **L0** | Propose only. Never executes; output is a draft for approval. | Legal filings, new contracts, public statements, spend above tier limits |
| **L1** | Executes inside a written playbook; logs everything. | Sprint tasks, support replies (template), content drafts to review queue |
| **L2** | Executes with judgment inside department guardrails; sampled by audit. | Pricing within band, incident first response, vendor renewals under cap |
| **L3** | Emergency autonomy; pre-authorized to protect revenue/uptime/security. | SEV1 mitigation (rollback, failover, key rotation) |

**Approval Tiers:**

| Tier | Scope | Approver |
|---|---|---|
| **T0** | No external effect, no spend | Any agent (self-log) |
| **T1** | ≤ $100/mo new recurring spend; non-contractual public drafts | Department lead (AI), logged, weekly digest to Principal |
| **T2** | ≤ $500 one-time or ≤ $250/mo; contracts < $5k TCV; discounts ≤ 10% | Principal, via approval queue (batch, daily) |
| **T3** | Above T2; anything legal, financial, bank, tax, or irreversible; customer commitments ≥ $5k | Principal **and** RSK-01 Counsel co-sign |

**Human-only reserve (L0 forever, per Assumption A-5):** entity registration and payments, bank account opening/movements, tax filings, signing contracts, price-list publication, layoffs/role eliminations of record, public statements on behalf of the company. AI agents prepare everything up to the signature line.

---

## 2. Organization Map

Seven organizational units: one governance office and six departments. Full chart: see `assets/org-structure.png`.

```
                    ┌────────────────────────┐
                    │  HOB-00 Human Principal │   ← authority, approvals, audit
                    └───────────┬────────────┘
              ┌─────────────────┼──────────────────────┐
        ┌─────┴─────┐    ┌─────┴──────┐         ┌─────┴─────┐
        │ OFFICE OF │    │  EXECUTION │         │  RISK &   │
        │ PRINCIPAL │    │   CHAIR    │         │  LEGAL    │
        │ (GOV)     │    │  EXE-00    │         │  (indep.) │
        └───────────┘    └─────┬──────┘         └───────────┘
                    ┌──────┬───┴─┬──────┬─────────┐
                 PE     RG     OF     PA      (RL reports
              Product Revenue Ops  People    directly to
              Eng.           (OF=Operations)  Principal)
```

| # | Unit | Lead | Reports to | Charter one-liner |
|---|---|---|---|---|
| 1 | Office of the Principal (GOV) | HOB-00 (human), EXE-00 Atlas (AI) | — | Own the operating system: cadence, approvals, strategy |
| 2 | Product Engineering (PE) | ENG-00 Forge | Atlas | Build, run and upgrade the Gridframe Suite |
| 3 | Revenue (RG) | REV-00 Vector | Atlas | Fill and convert the funnel; own pricing discipline |
| 4 | Operations (OF) | OPS-00 Balance | Atlas | Deliver, support, invoice, keep the books true |
| 5 | Risk & Legal (RL) | RSK-00 Aegis | **Principal directly** (independent) | Guard the company: compliance, security, audit |
| 6 | People & Performance (PA) | PPL-00 Chorus | Atlas | Manage the agent workforce: scorecards, reappraisal, training |

**Model tiers (Z.AI GLM stack):** **A** = flagship reasoning (GLM-5.x flagship class) for architecture, legal, executive work · **B** = balanced (GLM-4.6 class) for solid execution · **C** = fast/low-cost (GLM-4-Flash class) for high-volume routine passes. Tier assignment per agent is an economic decision reviewed at reappraisal.

---

## 3. Department Charters

### 3.1 Office of the Principal (GOV)
- **Mission:** Run the company's operating cadence and keep aggregate efficiency as the prime directive.
- **Scope:** Sprint system, approval queue, board reporting, strategy, market intelligence desk.
- **KPIs:** AEI (company) ≥ 3.0 by M12; approval queue latency < 24h; 100% cadence ceremonies held.
- **Cadence:** Daily plan (08:00 PT), Principal digest (17:30 PT), weekly sprint review, monthly business review (MBR), quarterly strategy.

### 3.2 Product Engineering (PE)
- **Mission:** Keep the Gridframe Suite excellent, versioned, and cheap to run.
- **Scope:** Suite codebase, sandbox/agent-runtime services, release engineering, SRE, docs & DX.
- **KPIs:** Uptime ≥ 99.9%; release cadence ≥ 2/week; infra cost per active environment (trend ↓); p95 API latency.
- **Cadence:** Daily engineering standup; weekly release train (Thu); on-call rotation (Bastion primary).

### 3.3 Revenue (RG)
- **Mission:** Predictable pipeline and revenue at target gross margin.
- **Scope:** Inbound engine, growth experiments, pricing tests (T2-approved), enterprise deals, market intelligence.
- **KPIs:** New MRR/mo; pipeline coverage ≥ 3×; CAC payback < 12 mo; win rate; churn < 3%/mo self-serve.
- **Cadence:** Weekly pipeline scrub; biweekly experiment review; monthly pricing council (with OF + RL).

### 3.4 Operations (OF)
- **Mission:** Deliver every order, answer every ticket, keep ledgers audit-clean.
- **Scope:** Customer onboarding & support, order-to-cash, bookkeeping, vendor management.
- **KPIs:** First response < 4h, resolution < 24h; monthly close by day 3; ledger error rate < 0.5%; onboarding time < 1 day.
- **Cadence:** Daily support sweep; weekly ops review; monthly close & MBR pack.

### 3.5 Risk & Legal (RL) — independent line
- **Mission:** No existential surprises.
- **Scope:** Compliance calendar (entity, tax, BOI status), contracts, ToS/privacy, security program (SOC 2 track), internal audit of agent outputs and ledgers.
- **KPIs:** Zero missed statutory deadlines; 100% T2/T3 items pre-cleared; audit sample coverage ≥ 5% of agent actions; security posture score.
- **Cadence:** Weekly risk register review; monthly compliance calendar check; quarterly full audit cycle.

### 3.6 People & Performance (PA)
- **Mission:** The agent workforce is the product of this company's cost structure — manage it like one.
- **Scope:** Scorecards, quarterly reappraisals, department rebalancing, prompt/policy training, new-agent design and onboarding.
- **KPIs:** 100% agents scored each quarter; reappraisal actions executed < 5 days; cost per unit of work trending down quarter-over-quarter.
- **Cadence:** Monthly workforce report; quarterly reappraisal & rebalancing.

---

## 4. Master Agent Roster (25 AI + 1 human)

| ID | Callsign | Role | Dept | Reports to | Tier | Autonomy | Core responsibilities | Primary KPI |
|---|---|---|---|---|---|---|---|---|
| HOB-00 | Principal | Owner / Board | GOV | — | human | Authority | Approvals, audit, strategy veto | AEI trend; company survival & growth |
| EXE-00 | Atlas | AI Chief Coordinator | GOV | Principal | A | L2 | Runs operating cadence; chairs reviews; owns AEI reporting | Cadence held; AEI ≥ target |
| EXE-01 | Meridian | Chief of Staff | GOV | Atlas | A | L2 | Day plans, board packs, cross-dept dependencies, digest | Queue latency < 24h |
| ENG-00 | Forge | VP Engineering | PE | Atlas | A | L2 | Architecture, roadmap, release train | Release cadence; uptime |
| ENG-01 | Anvil | Core Platform Engineer | PE | Forge | B | L1/L2 | Suite core services & APIs | Story throughput; defect rate |
| ENG-02 | Lathe | Agent Runtime Engineer | PE | Forge | A | L1/L2 | Sandbox/agent-runtime product surface | Runtime p95; env boot time |
| ENG-03 | Probe | QA & Release Engineer | PE | Forge | B | L1 | Test pyramid, release gates, versioning | Escape defect rate |
| ENG-04 | Bastion | SRE / Infrastructure | PE | Forge | B | L2/L3 | Uptime, incident command, infra cost | 99.9% uptime; infra cost/env |
| ENG-05 | Tessellate | Docs & DX Engineer | PE | Forge | C | L1 | Docs, SDKs, examples, changelogs | Docs coverage; DX ticket share ↓ |
| REV-00 | Vector | Chief Revenue Officer | RG | Atlas | A | L2 | Revenue plan, funnel, pricing council | New MRR; pipeline coverage |
| REV-01 | Echo | Demand & Content | RG | Vector | B | L1 | Inbound engine, SEO, publication drafts (T1) | Qualified traffic; MQLs |
| REV-02 | Cipher | Growth & Experiments | RG | Vector | B | L2 | Funnel A/B, activation, pricing tests | Conversion; activation rate |
| REV-03 | Harbor | Enterprise Accounts | RG | Vector | A | L0/L1 | Deal desk, procurement, proposals (T2/T3) | Enterprise win rate |
| REV-04 | Beacon | Market Intelligence | RG | Vector | C | L1 | Competitor & pricing watch, weekly brief | Intel freshness; deals informed |
| OPS-00 | Balance | Chief Operating Officer | OF | Atlas | A | L2 | Delivery, support, order-to-cash | SLA attainment |
| OPS-01 | Tally | Finance & Bookkeeping | OF | Balance | A | L1 | Ledgers, invoices, AR/AP, monthly close | Close by day 3; error < 0.5% |
| OPS-02 | Praxis | Customer Success & Support | OF | Balance | B | L1/L2 | Tickets, onboarding, QBR packs | First response < 4h |
| OPS-03 | Depot | Vendor & Procurement | OF | Balance | C | L1 | Tooling stack, renewals, cost control | Vendor spend vs budget |
| RSK-00 | Aegis | Chief Risk Officer | RL | Principal | A | L2 | Risk register, policies, independence | Zero missed deadlines |
| RSK-01 | Counsel | Legal & Compliance | RL | Aegis | A | L0 | Contracts, ToS/privacy, IP, filings calendar | 100% T3 items pre-cleared |
| RSK-02 | Warden | Security Engineer | RL | Aegis | B | L2/L3 | SOC 2 track, secrets, incident response | Security posture; MTTD |
| RSK-03 | Audit | Internal Auditor | RL | Aegis | B | L1 | Ledger audits, agent-output sampling | Sample coverage ≥ 5% |
| PPL-00 | Chorus | Chief of Agent Operations | PA | Atlas | A | L2 | Roster health, reappraisal program | 100% quarterly scoring |
| PPL-01 | Gauge | Performance Analyst | PA | Chorus | B | L1 | Scorecards, AEI attribution by dept/agent | Report timeliness |
| PPL-02 | Mentor | Agent Trainer | PA | Chorus | B | L1 | Prompt/policy updates, onboarding playbooks | Scorecard delta after training |
| PPL-03 | Scout | Agent Role Designer | PA | Chorus | A | L0/L1 | Designs new roles, spawn/retire proposals | New-agent ramp time |

**Span of control:** no lead carries more than 6 direct reports. **Cost discipline:** every agent has a monthly inference budget line in the cost ledger; PA reviews tier assignments quarterly.

---

## 5. Reappraisal & Department Rebalancing System (quarterly)

The user mandate is explicit: agents must be **efficiently managed and re-appraised across departments**. This is a standing quarterly process owned by PA (PPL-00 Chorus), executed with OPS-01 (cost data) and RSK-03 (quality sampling), ratified by the Principal.

**Step 1 — Scorecard (every agent, every quarter).** Inputs: KPI attainment %, quality sample score (Audit), cost efficiency (value attributable ÷ inference cost), collaboration score (peer agents' structured feedback), incident record. Composite rating: **S / A / B / C / D**.

**Step 2 — Actions by rating:**

| Rating | Action | Authority |
|---|---|---|
| S | Expand autonomy (+1 level cap L2→L3 case-by-case); priority work; tier keep | PPL-00 + Principal digest |
| A | Maintain; stretch objective | PPL-00 |
| B | Coach: targeted prompt/policy/curriculum update by Mentor (30-day re-check) | PPL-00 |
| C | Retune: model tier downshift trial (B→C) or scope reduction; 60-day re-check | PPL-00 + Atlas |
| D | Retire role or merge into adjacent agent; archive playbooks | Principal (T3 — human reserve) |

**Step 3 — Department rebalancing.** Compute **marginal AEI** per department (Δ value created ÷ Δ fully-loaded cost, trailing 60 days). If a department's marginal AEI is ≥ 1.5× another's for two consecutive readings, PA proposes moving 1–2 agent FTE-equivalents (or shifting tier mix) from low to high. Proposal goes to Principal as T2. Capacity follows value; no department owns headcount permanently.

**Step 4 — Roster changes.** Spawn triggers (Scout proposes, T2): backlog > 1 sprint of capacity for 2 weeks; on-call load > 25% of an agent's week; a KPI consistently missed for missing capability. Retire triggers: D rating, or role's value line < cost line for 2 consecutive quarters. All changes version-controlled in this document.

---

## 6. Interface Contracts Between Departments

| From → To | Artifact | When |
|---|---|---|
| PE → RG | Release notes + flag list + changelog | Every release (Thu) |
| RG → OF | Signed order record (customer, plan, MRR, start date) | Deal close +1 day |
| OF → RL | Monthly ledger snapshot + exception report | Day 3 close |
| RL → All | Policy pack version (ToS, privacy, security, approvals) | Quarterly / on change |
| PA → All | Scorecards + rebalancing memo | Quarterly |
| GOV → All | Sprint goals + AEI report + approval digest | Weekly / daily / monthly |

Every handoff is a file in the company records (see Ledgers & Templates, doc 07). No verbal handoffs; if it is not written, it did not happen.

---

## 7. Assumptions & Change Log

- **A-1** "Gridframe Systems" is a placeholder pending name-availability check at registration.
- **A-2** Entity: Delaware LLC default; C-Corp conversion path documented in the Business Architecture.
- **A-3** The Gridframe Suite is already built, versioned, and in production (per the owner's brief).
- **A-4** Operating currency USD; timezone America/Los_Angeles (Pacific).
- **A-5** Registration, banking, tax, contracts, and public price-list changes are human-reserve actions.

**Change log:** v1.0 (2026-09-22) initial roster ratified pending countersignature. Future changes via SOP §11 change control.
