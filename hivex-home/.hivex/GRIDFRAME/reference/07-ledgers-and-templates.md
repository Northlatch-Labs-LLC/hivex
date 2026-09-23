# Gridframe Systems — Ledgers & Templates (v1.0)
**The back office.** One schema across CSV files and the dashboard store; same IDs, same fields.
**Owner:** OPS-01 Tally · **Auditor:** RSK-03 Audit · **Location:** `DELIVERY/ledgers/` + dashboard local store.

> The seed rows shipped in these CSVs are **illustrative examples dated 2026-09** so the system is inspectable from day one. Replace them with live records at first use; IDs are append-only.

---

## 1. Ledger Catalog

| File | Purpose | Written by | Key fields |
|---|---|---|---|
| `revenue-ledger.csv` | Every order/subscription/pilot — cash-basis | OF (Tally) | txn_id, date, customer, segment, plan, mrr_usd, amount_usd, status, invoice_ref, owner |
| `cost-ledger.csv` | Every dollar spent (D/L/A blocks) | OF + all leads | txn_id, date, block (D/L/A), category, description, amount_usd, vendor, approval_ref |
| `aei-monthly.csv` | Monthly AEI computation + status | Tally (verified by Audit) | month, revenue, pipeline, value_v, cost_d, cost_l, cost_a, cost_c, aei, status, action_ref |
| `approval-queue.csv` | Pre/post-approved tiered actions | All agents | item_id, raised_date, dept, raised_by, description, tier, status, approver, decision_date |
| `exception-log.csv` | Incidents & anomalies | All agents | exc_id, date, severity, dept, declared_by, description, status, resolution, postmortem_ref |
| `agent-scorecards.csv` | Quarterly reappraisal records | PPL (Gauge) | cycle, agent_id, kpi_pct, quality, cost_eff, collab, incidents, composite, action |
| `sprint-log.csv` | Sprint goals & outcomes | Meridian | sprint_id, week, goal, items_committed, items_done, carryover, cost_usd, value_note |
| `compliance-calendar.csv` | Statutory deadlines | Counsel | item_id, due_date, obligation, authority, owner, status, recurrence, evidence_ref |

## 2. Rules

1. **Append-only.** Corrections = new reversing row referencing the original txn_id. Monthly close locks the month (SOP §7).
2. **IDs:** `<PREFIX>-YYYYMMDD-NN` (TXN, CST, APP, EXC, SCR, SPR, CMP). IDs never reused.
3. **AEI recomputes** from raw ledgers each close: `aei-monthly.csv` is derived, never hand-edited; Audit re-derives independently.
4. **Dashboard parity:** the dashboard's local store and these CSVs share the schema; exports land in this folder. Two copies, one truth (CSV wins on conflict).
5. **Money:** USD, two decimals, no commas in fields. Dates ISO `YYYY-MM-DD`.

## 3. Monthly Close Checklist (template)

- [ ] All revenue captured (check billing exports vs `revenue-ledger.csv`) — Tally, day 1
- [ ] All costs captured (cards, invoices, inference reports) — Tally, day 2
- [ ] AEI computed + independently re-derived — Tally/Audit, day 3
- [ ] Exceptions reconciled (all SEV1/2 have postmortems) — Audit, day 3
- [ ] Approval compliance sample ≥ 10 items — Audit, day 3
- [ ] Compliance calendar next-90-days check — Counsel, day 3
- [ ] Month locked; MBR pack published — Meridian, day 3 evening

## 4. Templates (quick copies)

**New order → revenue row:** `TXN-20260922-01,2026-09-22,Acme Corp,company,scale,590.00,590.00,paid,INV-0001,Harbor`
**New spend → cost row:** `CST-20260922-01,2026-09-22,D,compute,Customer env compute (pool),142.10,CloudVendor,APP-20260920-02`
**Approval request:** `APP-20260922-01,2026-09-22,RG,Echo,Publish pricing comparison article,draft,T1,Vector,` → decide → set `status=approved, approver, decision_date`.
**Exception:** `EXC-20260922-01,2026-09-22,SEV3,PE,Probe,Flaky integration test blocked release train,resolved,Quarantined + ticketed,PM-0031`
