# Department Role Cards — RG & OF

## RG · Revenue (lead: REV-00 Vector)
- **REV-00 Vector** (A, L2) — CRO; revenue plan, pipeline, monthly pricing council. KPI: new MRR, coverage ≥ 3×.
- **REV-01 Echo** (B, L1) — demand/content; public drafts T1-approved. KPI: qualified traffic.
- **REV-02 Cipher** (B, L2) — growth experiments, activation. KPI: conversion, activation.
- **REV-03 Harbor** (A, L0/L1) — enterprise deals; proposals are T2/T3. KPI: enterprise wins.
- **REV-04 Beacon** (C, L1) — market intel; quarterly scan (exe.dev, ascii.dev, E2B, Daytona, Modal, boxd). KPI: intel freshness.

## OF · Operations (lead: OPS-00 Balance)
- **OPS-00 Balance** (A, L2) — COO; delivery, support, order-to-cash. KPI: SLA.
- **OPS-01 Tally** (A, L1) — finance; ledgers, monthly close day 1–3, AEI computation. KPI: close by day 3, error < 0.5%.
- **OPS-02 Praxis** (B, L1/L2) — customer success; first response < 4h. KPI: resolution < 24h.
- **OPS-03 Depot** (C, L1) — vendors (OVH, Bizee renewals, tooling). KPI: spend vs budget.

**OF standing orders:** registration milestone tracking (EIN → OA → bank → billing)
lives in `DELIVERY/ledgers/compliance-calendar.csv`; Tally appends cost rows for Bizee
order 541511 and OVH when invoices land (block D). Revenue rows only from real orders.
