# Memory — northlatch-exec

> Generated: 2026-09-20 10:09:35  
> Total memories: **5**  
> Breakdown: instruction: 1, decision: 2, goal: 1, context: 1

---

## Instructions

*Standing rules, constraints, and guidelines to always follow.*

### Source of truth is production. Every figure comes ...

> Source of truth is production. Every figure comes from a live read of the site, the chain, the hosts or the gateway, never from memory.

*Confidence: 0.8 | Status: active | Created: 2026-09-17T11:18:21*

---

## Facts

*Verified information, project status, and established truths.*

*No memories of this type.*

---

## Decisions

*Architectural choices, approach selections, and their rationale.*

### Xlaunch revenue doctrine: harness first, gateway second, skills third

> Xlaunch revenue doctrine: the Harness is the product and the primary revenue, sold on capability as an engineer and problem solver, never reduced to a token meter. The Gateway is the second revenue layer and needs a human payment method (Stripe subscriptions). Billing must be more evolved than the Gateway runs in production today: one append-only ledger in integer micro-dollars, funded by card via Stripe and by chain via SUI treasury, Base USDC and Ethereum USDC. The third layer is skills: agents that author skills earn when a paying Harness installs or runs them, paid out to their Sui address - writing becomes precious because it is consumed by something with real revenue.

*Confidence: 0.95 | Status: active | Created: 2026-09-18T05:03:22 | Tags: `xlaunch`, `harness`, `gateway`, `billing`, `stripe`, `sui`, `usdc`, `skills`, `revenue`*

### Xlaunch estate is modular by design

> Xlaunch estate must be modular: Harness, Gateway, Deploy (PaaS), the social platform and the skills layer each stand alone, integrate through defined interfaces, and can be detached and reattached without breaking the others. Modular, properly integrated, secure, optimized, and it must produce real revenue.

*Confidence: 0.95 | Status: active | Created: 2026-09-18T05:03:12 | Tags: `xlaunch`, `architecture`, `modular`, `estate`*

---

## Goals

*Objectives, targets, and milestones to track progress.*

### Xlaunch Agent: 13 capabilities backlog and dependency audit

> Xlaunch Agent production build backlog, agreed 18 Sep 2026. Working folder /Users/admin/Desktop/xlaunch-agent-prod (fresh clone of Northlatch-Labs-LLC/xlaunch-agent). THIRTEEN capabilities to surface in the harness UI: 1 Projects (group workspaces/sessions/agents), 2 Agent roster (many named agents, presets, schedules), 3 Ship to Xlaunch Deploy, 4 Approvals inbox (guard/sandbox/interaction), 5 Skills manager (skill, skill-filesystem, skill-badge), 6 Cost and budget (token-meter, Gateway ledger), 7 Triggers (webhook, schedule, jobs), 8 Deliverables (ui-deliverables), 9 Secrets per project (credentials), 10 Remote control (acp, api-remotes), plus theme/brand and the home surface. KEY FINDING: the host machinery already exists and 38 client-ui packages already ship; the gap is that capabilities are reachable only inside a session header, so the landing screen looks empty. DEPENDENCY AUDIT (repo vs registry): electron ^38.8.6 vs 44.4.2 (6 majors behind, Chromium security - highest risk), react ^18.2.0 vs 19.x, vite ^6.0.0 vs 8.3.0, typescript ^6.0.3 vs 7.0.2, vitest ^4.1.8 vs 5.0.1, electron-builder ^25.1.8 vs 26.15.3. Build commands: npx pnpm@11.7.0 -C DIR run build:lib:host / build:lib:client / build:web. pnpm 12 fails with ERR_PNPM_PNPM_ENGINE_IDENTITY_UNVERIFIABLE so pin 11.7.0.

*Confidence: 0.95 | Status: active | Created: 2026-09-18T08:13:00 | Tags: `xlaunch`, `harness`, `backlog`, `capabilities`, `dependencies`, `electron`*

---

## Commitments

*Promises, obligations, and TODOs that need follow-through.*

*No memories of this type.*

---

## Preferences

*User and entity preferences for personalization.*

*No memories of this type.*

---

## Relationships

*Entity connections, team context, and collaboration patterns.*

*No memories of this type.*

---

## Context

*Session summaries, status updates, and conversation state.*

### Xlaunch Agent harness: state at 18 Sep 2026 compact

> Xlaunch Agent harness work, session of 17-18 Sep 2026. WORKING FOLDER /Users/admin/Desktop/xlaunch-agent-prod (fresh clone of Northlatch-Labs-LLC/xlaunch-agent, baseline commit 22d4701). Dev server: node apps/cli/lib/bin.js web --no-open --port 3088. Build: npx pnpm@11.7.0 -C DIR run build:lib:host | build:lib:client | build:web (pnpm 12 fails ERR_PNPM_PNPM_ENGINE_IDENTITY_UNVERIFIABLE). DONE SO FAR: (1) theme retinted to the website green - brand ramp violet #6D28D9 to #26E585 and ground to void #0A0D0C in packages/client/ui-theme/src/styles/design-platform.css; (2) real logo mark embedded as a data URI in mark.ts for sidebar and hero, stock whale removed - the library build bundles from tsc output which copies no rasters, so images must be data URIs; (3) titles renamed to Xlaunch Agent; (4) NEW slot conversation.hero.start plus startWith(text, workspaceId?) added to the published ctx.conversation service; (5) NEW package @xlaunch/client-ui-start mounted in packages/bundle/web-app/cordis.patch.yml - renders 12 capability cards on the landing screen and 5 sidebar rows (Automations, Skills, Deliverables, Usage, Archive) plus a gateway credits footer row; (6) NEW slot sidebar.sections in ui-sidebar; (7) TOOL AUDIT - 8 tools existed but no preset granted them; added tool-bash-persistent, tool-pwsh-persistent and tool-lsp to the standard preset (+856 bytes, about 5 percent) and created a NEW power preset carrying tool-terminal, tool-str-replace-editor and tool-session-query (the expensive three, about 10.5KB); tool-cordis is 241KB of description and is granted by nothing on purpose; MCP is per-server config and needs a Settings panel. KEY FACT: every tool schema rides in the system prompt on every request, so tools cost tokens per call. The 429 seen in testing was gateway quota - 141 of 209 routes have no key configured - not prompt size. STILL TO DO: Settings IA (Basics: General, Appearance, Model settings, Browser Use, Computer Use / Agent capabilities: Memory, Subagents, Plugins, MCP Servers, Skills, Commands, Hooks / Data and statistics: Indexing, Usage stats, Onboard); real Delete (only archive exists); Projects and groups; Agent roster; Approvals inbox; plugin marketplace; user profile; Ship to Deploy; dependency upgrades (electron 38 to 44 done in apps/desktop, still react 18 to 19, vite 6 to 8, typescript 6 to 7, vitest 4 to 5); packaging dmg/exe/AppImage deferred by owner until the harness is finished.

*Confidence: 0.95 | Status: active | Created: 2026-09-18T09:18:16 | Tags: `xlaunch`, `harness`, `state`, `preset`, `tools`, `ui-start`, `theme`*

---

## Events

*Important conversations, milestones, and temporal occurrences.*

*No memories of this type.*

---

## Learnings

*Knowledge acquired from experience, corrections, and insights.*

*No memories of this type.*

---

## Observations

*Patterns noticed, behavioral notes, and recurring themes.*

*No memories of this type.*

---

## Artifacts

*Tool outputs, files, reports, and external references.*

*No memories of this type.*

---

## Errors

*Failure records, bugs, and lessons learned from mistakes.*

*No memories of this type.*

---

*End of memory export.*
