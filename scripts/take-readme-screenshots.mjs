// Take the README screenshots from the live office (loopback 7912).
// Usage (from web/ so playwright resolves): bun ../scripts/take-readme-screenshots.mjs
import { chromium } from "@playwright/test";
import { mkdirSync } from "node:fs";

const BASE = process.env.HIVEX_SHOT_BASE ?? "http://127.0.0.1:7912";
const OUT = "docs/screenshots";
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const page = await browser.newPage({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
});

async function mounted() {
  await page.waitForFunction(
    () => {
      const root = document.getElementById("root");
      return root && root.children.length > 0 && !document.getElementById("skeleton");
    },
    { timeout: 15_000 },
  );
}

async function shot(path, name, settleSel = null) {
  await page.goto(`${BASE}/#${path}`, { waitUntil: "domcontentloaded" });
  await mounted();
  if (settleSel) {
    await page.locator(settleSel).first().waitFor({ timeout: 10_000 }).catch(() => {});
  }
  await page.waitForTimeout(900);
  await page.screenshot({ path: `${OUT}/${name}.png` });
  console.log(`shot ${name}`);
}

async function settingsShot(navName, headline, name) {
  await page.goto(`${BASE}/#/apps/settings`, { waitUntil: "domcontentloaded" });
  await mounted();
  await page.getByRole("button", { name: navName, exact: true }).first().click();
  await page
    .locator("h2", { hasText: headline })
    .first()
    .waitFor({ timeout: 10_000 });
  await page.waitForTimeout(700);
  await page.screenshot({ path: `${OUT}/${name}.png` });
  console.log(`shot ${name}`);
}

await shot("", "office-home", "[data-testid='sidebar-section-work']");
await shot("/tasks", "tasks");
await settingsShot("API Keys", "API Keys", "credentials-zai");
await settingsShot("General", "Default runtime for new bots", "runtimes-zai-code");
await shot("/board", "gridframe-board");

await browser.close();
console.log("done");
