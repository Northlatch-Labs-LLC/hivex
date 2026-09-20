# web/e2e

Playwright smoke tests against the real hivex web UI. Specs are split across
fresh-install and post-onboarding shell phases:

| Spec | Phase | Precondition |
|---|---|---|
| `tests/wizard.spec.ts` | fresh install | **no** `~/.hivex/onboarded.json` — hivex serves the onboarding wizard |
| `tests/smoke.spec.ts` | post-onboarding shell | `~/.hivex/onboarded.json` is **seeded** — hivex serves the shell, with sidebar + bot panel |
| `tests/app-routes.spec.ts` | post-onboarding shell | seeded shell; app routes must render independently without leaking into each other |
| `tests/route-matrix.spec.ts` | post-onboarding shell | seeded shell; every canonical route and dropped legacy alias must render the expected surface |

CI runs both in `.github/workflows/ci.yml :: web-e2e` by booting hivex twice (once with each precondition).

## Running locally

Use `web/e2e/run-local.sh`. It pins `HIVEX_RUNTIME_HOME` to a per-run tempdir so your real `~/.hivex/onboarded.json` and `~/.hivex/team/broker-state.json` are never touched.

```bash
# both phases (wizard, then shell — what CI does)
web/e2e/run-local.sh

# just one
web/e2e/run-local.sh wizard
web/e2e/run-local.sh shell

# alternate ports if 27891 collides locally
PORT=37891 web/e2e/run-local.sh
```

The script:

- Builds `web/dist` and the `hivex` binary if missing.
- Pins `HIVEX_RUNTIME_HOME` to a per-run tempdir, sandboxing all on-disk state.
- For the shell phase, seeds `<RUNTIME_HOME>/.hivex/onboarded.json` (same JSON CI writes — see `ci.yml :: seed onboarding state`) before launching.
- Launches hivex on `27891` (configurable) and `27890` (broker port = web port − 1) so it never collides with a developer's normally-running `7891` hivex.
- Cleans up on exit (kills hivex, removes the tempdir).

## Why this script exists at all

The smoke spec assumes `onboarded.json` is seeded — without it, hivex serves the wizard and `.bot-panel` (a shell-only component) never mounts, so the tests fail with a 10s locator timeout that looks like a UI regression but is really a missing precondition. The CI workflow handles this in shell; this script is the local-friendly equivalent so devs don't have to read the workflow YAML to figure out the contract.
