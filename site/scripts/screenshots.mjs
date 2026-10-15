// Screenshots of the site for PR review (issue #72).
// Usage: node scripts/screenshots.mjs <baseURL> <outDir>
// Writes one PNG per shot plus manifest.json, which lists them in display order.
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { chromium } from "playwright";

const [baseURL, outDir] = process.argv.slice(2);
if (!baseURL || !outDir) {
  console.error("usage: screenshots.mjs <baseURL> <outDir>");
  process.exit(2);
}

const viewports = {
  desktop: { width: 1440, height: 900 },
  phone: { width: 390, height: 844 },
};
const pages = [
  { name: "home", label: "/", path: "/", status: 200 },
  { name: "404", label: "404", path: "/__screenshots-not-found", status: 404 },
];

// A wrong status means the shot shows the wrong page, such as an SPA fallback
// served for the 404 path, so stop instead of capturing it.
async function open(page, path, status) {
  const res = await page.goto(path, { waitUntil: "networkidle" });
  if (res.status() !== status) {
    throw new Error(`${path} returned ${res.status()}, expected ${status}`);
  }
}

// The page settles once web fonts are in, so text metrics don't shift between runs.
async function settle(page) {
  await page.evaluate(() => document.fonts.ready);
}

// Playwright's fullPage capture draws the sheet's fixed vignette against the
// original viewport, which leaves a seam at its bottom edge. Growing the
// viewport to the capture height instead renders it as a window that tall would.
async function shoot(page, path, height) {
  const size = page.viewportSize();
  height ??= await page.evaluate(() => document.documentElement.scrollHeight);
  await page.setViewportSize({ width: size.width, height });
  await page.evaluate(() => new Promise(requestAnimationFrame));
  await page.screenshot({ path, animations: "disabled" });
  await page.setViewportSize(size);
}

const browser = await chromium.launch();
const manifest = [];
try {
  await mkdir(outDir, { recursive: true });

  // Reduced motion shows Fig. 1's whole transcript and turns off every intro
  // and scroll-driven animation, so full-page shots are the same on every run.
  for (const [viewport, size] of Object.entries(viewports)) {
    const context = await browser.newContext({ baseURL, viewport: size, reducedMotion: "reduce" });
    const page = await context.newPage();
    for (const p of pages) {
      await open(page, p.path, p.status);
      await settle(page);
      const file = `${p.name}-${viewport}.png`;
      await shoot(page, join(outDir, file));
      manifest.push({ file, label: `${p.label} · ${viewport}` });
    }
    await context.close();
  }

  // One shot as a visitor sees it with motion on. It runs from the top of the
  // page to the end of Fig. 1 and no further: the sections after it use
  // scroll-driven animations, which have no fixed state in a capture.
  const context = await browser.newContext({ baseURL, viewport: viewports.desktop, reducedMotion: "no-preference" });
  const page = await context.newPage();
  await open(page, "/", 200);
  await settle(page);
  // Stop puts the terminal in the same state as a finished replay. A real click
  // would scroll the button into view, so dispatch it to keep the page at the top.
  await page.getByRole("button", { name: "■ Stop" }).dispatchEvent("click");
  await page.getByRole("button", { name: "↻ Replay" }).waitFor();
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.timeline instanceof DocumentTimeline)
        .map((a) => a.finished),
    ),
  );
  // The page is still at the top, so the box is in page coordinates.
  const fig = await page.locator(".figure").first().boundingBox();
  const file = "home-desktop-motion.png";
  await shoot(page, join(outDir, file), Math.ceil(fig.y + fig.height + 48));
  manifest.push({ file, label: "/ · desktop · motion" });
  await context.close();

  await writeFile(join(outDir, "manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
} finally {
  await browser.close();
}
