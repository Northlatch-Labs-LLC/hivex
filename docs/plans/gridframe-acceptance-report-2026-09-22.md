# Gridframe Acceptance Report — 2026-09-22 (G5)

Spec: `/Users/admin/Desktop/11-hivex-build-instruction.md` §10. All 10 tests executed
in-repo against the real roster (`hivex-home/.hivex/GRIDFRAME/roster.json`) and real
seeds (`hivex-home/.hivex/GRIDFRAME/reference/ledgers/`).

Runners: `internal/gridframe/acceptance_g5_test.go` (tests 1–5, 363 ln) and
`internal/gridframe/acceptance_g5b_test.go` (tests 6–9, 254 ln).

## Results

Commands (this G5 slice): `go test ./internal/gridframe/ -run 'TestAcc[1-5]' -v` → 5/5
PASS; `go test ./internal/gridframe/ -run 'TestAcc[6-9]' -v` → 4/4 PASS;
`go test ./internal/gridframe/...` → ok (69 tests); `go vet ./internal/gridframe/` clean.

| # | Test | Result | Evidence |
|---|------|--------|----------|
| 1 | Roster integrity | PASS | 26 entries = 25 agents + HOB-00; dept ∈ 6, tier ∈ {A,B,C}, autonomy parses for all; members → dept lead; ENG/REV/OPS/PPL leads → EXE-00; EXE-00, RSK-00 → HOB-00 (RL independent line) |
| 2 | Ledger round-trip | PASS | 8 tables: seeded export byte-equals seed CSV; export→import→export byte-stable; re-append same ID → `ErrEditExisting`; `AppendReversal` accepted with reversal link |
| 3 | AEI correctness | PASS | `DeriveAEI("2026-09", pipeline 12000)`: rev 329.00, D 170.10, L 340.00, A 137.60, C 647.70, V 1529.00, AEI 2.36 healthy; `ReverifyAEI` on published seed row agrees, 0 diffs |
| 4 | Governance gate | PASS | T2 ($300) → Block pre-execution, pending row approver HOB-00; agent signer → `ErrWrongSigners`; human event → approved+dated. Reserve "publish-price-list" → Reject; T3 dual-sign without Human → `ErrNeedsHuman` |
| 5 | Cadence | PASS | 9 jobs numbered 1–9, crons parse + NextRun, tz America/Los_Angeles; trigger job 1 → `day/2026-09-23.md`, job 2 → `digests/2026-09-23.md`; job 7 (OPS-01+RSK-03) appends aei row 2026-09/2.36/healthy, re-derivation agrees, no freeze; forced AEI<1.0 (Oct cost 500 D, no revenue, pipeline 0) → AEI 0.00 survival, `freeze-t1-discretionary-spend` in artifact, `SpendFreezeActive`=true |
| 6 | Exception path | PASS | SEV2 `Declare` → open EXC row; SEV1 declare → both rows in `/gridframe/digest` open_exceptions (mock; latency 0 ≤ 30 min) |
| 7 | Board | PASS | add-row POST → 201 + persists in listing; export bytes == store `Export()` incl. new row; decide approve → queue row approved+dated, item leaves digest decisions |
| 8 | State | PASS | `MilestoneDone("CMP-0005","12-3456789")` → formation.ein set, milestone dropped, Save/Load round-trip; compliance CMP-0005 → done, overdue=false |
| 9 | Audit + composite | PASS | 3 test actions (2×T1 + 1 T2 approved) → `Sample("2026-09")` ≥1 id ≥5%; violation verdict + `Violations()` lists it; unsampled ID rejected; `Score(ENG-01)` composite/grade/action; SCR row appended |
| 10 | Docs shipped | PARTIAL | All files present read-only 0444: 4 normative .md + state.json + 3 role cards + hivex-mapping.json (26/26 non-empty bindings). MISSING: no help/about reference — `grep -rn "hivex-mapping\|GRIDFRAME/reference\|02-organization-agent-roster\|07-ledgers" web/src cmd internal apps` → 0 matches |

## Deviations (honest)

- **Test 10 "referenced from help/about" not met.** Docs ship; no platform surface
  references them. Fix = add a docs/help route or about-panel link.
- **Cadence job bodies are test-wired.** `Registry.SetBody` has no production caller
  (`grep -rn SetBody` → cadence.go + tests only). Engine functions are production code;
  scheduler→body wiring (and the scheduler daemon itself) is not built.
- **SEV1 notification is a mock.** Digest inclusion is immediate; no separate
  notification channel or ≤30-min timer exists.
- **Compliance status flip is caller-side** (state.go:97-101): no cadence-job-9 service
  wiring automates milestone→calendar updates yet.
- **Approval execution semantics.** "Executes after approval" is modeled as the queue
  row flipping approved (gate unblocks); there is no downstream executor to observe.
- Test-harness fixes during the run (not product changes): roster lead-report
  expectation corrected to EXE-00 per roster.json/spec §2; unused import; test 8
  initially pointed at an unseeded store.

## What remains human (by design)

- **All T2/T3 approvals** — HOB-00 sole T2 approver; T3 needs HOB-00+RSK-01 dual sign;
  engine enforces (`ErrWrongSigners`, `ErrNeedsHuman`). Not automatable per §4.
- **Human-reserve actions** — entity registration/payments, bank, tax, contract
  signatures, price-list publication, role eliminations, public statements: require an
  authenticated human action event; rejected even at T3 with dual signers.
- **Formation steps** (state.json, next_milestones): CMP-0005 EIN via IRS (due
  2026-09-24, Principal types SSN + Counsel prep), CMP-0006 sign operating agreement
  (2026-09-26), CMP-0007 bank account (2026-09-30), OVH-PROV bare-metal provisioning
  on delivery (ENG-04, manual per spec §11 out-of-scope OVH API).
- **Monthly verified pipeline input** — REV-00-verified close-pack field feeding the
  AEI engine; currently passed as a parameter, no workflow capture built.

## Verdict

9/10 PASS, 1 PARTIAL (test 10 docs-reference gap). Ledger, AEI, governance, cadence,
board, state, audit behaviors verified end-to-end against seed data.
