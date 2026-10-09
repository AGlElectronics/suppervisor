import AxeBuilder from "@axe-core/playwright";
import { chromium } from "playwright-core";

const browser = await chromium.launch({
  executablePath: process.env.CHROME_PATH ?? "/usr/local/bin/google-chrome",
  headless: true,
  args: ["--no-sandbox", "--disable-dev-shm-usage"],
});

const results = [];
async function audit(page, label) {
  const report = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  const blockers = report.violations.filter((violation) =>
    ["serious", "critical"].includes(violation.impact ?? ""),
  );
  results.push({
    label,
    violations: report.violations.length,
    seriousOrCritical: blockers.map((item) => item.id),
  });
  if (blockers.length > 0) {
    throw new Error(
      `${label} has serious accessibility violations: ${blockers
        .map((item) => item.id)
        .join(", ")}`,
    );
  }
}

const desktop = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  serviceWorkers: "block",
});
const page = await desktop.newPage();
await page.goto("http://127.0.0.1:4179", { waitUntil: "networkidle" });
await page
  .getByRole("heading", {
    name: "What should dinner do for you?",
  })
  .waitFor();
await page.getByText("Why this fits").click();
await page.getByText(/Assumed:/).waitFor();
await page.getByRole("button", { name: "Adjust" }).click();
await page.getByRole("dialog", { name: "What should change?" }).waitFor();
await page.getByLabel("Adjustment").fill("Use the hob, not the oven");
await page.getByRole("button", { name: "Cancel" }).click();
await page.getByRole("button", { name: "Recipe" }).first().click();
await page.getByRole("heading", { name: /Five calm minutes/ }).waitFor();
await page.getByRole("button", { name: /Open shopping/ }).click();
await page.getByRole("checkbox", { name: /orzo/i }).check();
await page.getByText(/1 of 5 tucked into the basket/).waitFor();
await page.getByRole("button", { name: "Cooking" }).first().click();
await page.getByRole("button", { name: /Next step/ }).click();
await page.getByRole("button", { name: /Next step/ }).click();
await page.getByRole("button", { name: /Dinner done/ }).click();
for (const dimension of ["Overall", "Taste", "Ease", "Reheating"]) {
  await page.getByRole("group", { name: dimension }).waitFor();
}
await page.getByRole("button", { name: "Taste: 5 out of 5" }).click();
await page.getByRole("button", { name: "make again" }).click();
await page
  .getByLabel("Anything worth remembering?")
  .fill("Bright, easy, and lunch-ready.");
await page.getByRole("button", { name: "Save rating" }).click();
await page.getByRole("button", { name: "Profile" }).first().click();
await page.getByRole("heading", { name: "Alan" }).waitFor();
await page.getByRole("button", { name: "History" }).first().click();
await page.getByRole("heading", { name: "Dinner’s paper trail" }).waitFor();
await page.getByRole("button", { name: "Manage" }).click();
for (const tab of [
  "profiles",
  "preferences",
  "meals",
  "ratings",
  "recipes",
  "decisions",
  "status",
  "corrections",
]) {
  await page.getByRole("tab", { name: tab }).click();
}
await page.getByRole("switch", { name: "Use dark theme" }).click();
await audit(page, "desktop management dark");
await page.screenshot({
  path: "/tmp/suppervisor-desktop.png",
  fullPage: true,
});
await desktop.close();

const mobile = await browser.newContext({
  viewport: { width: 390, height: 844 },
  serviceWorkers: "block",
  isMobile: true,
  hasTouch: true,
});
const mobilePage = await mobile.newPage();
await mobilePage.goto("http://127.0.0.1:4179", {
  waitUntil: "networkidle",
});
await mobilePage
  .getByRole("heading", {
    name: "What should dinner do for you?",
  })
  .waitFor();
const mobileNav = mobilePage.getByRole("navigation", {
  name: "Mobile navigation",
});
const mobileButtons = mobileNav.getByRole("button");
if ((await mobileButtons.count()) !== 6) {
  throw new Error(
    `expected 6 mobile destinations, got ${await mobileButtons.count()}`,
  );
}
await mobileNav.getByRole("button", { name: "History" }).click();
await mobilePage
  .getByRole("heading", { name: "Dinner’s paper trail" })
  .waitFor();
await mobilePage.getByRole("button", { name: "Open management" }).click();
await mobilePage.getByRole("tab", { name: "ratings" }).click();
await mobilePage.getByRole("columnheader", { name: "Taste" }).waitFor();
await audit(mobilePage, "mobile ratings");
await mobilePage.screenshot({
  path: "/tmp/suppervisor-mobile.png",
  fullPage: true,
});
await mobile.close();
await browser.close();

console.log(
  JSON.stringify(
    {
      result: "passed",
      flows: [
        "Tonight score explanation",
        "Adjust dialog",
        "Recipe prep",
        "Shopping progress",
        "Cooking completion",
        "Structured individual rating",
        "Profiles",
        "History",
        "All management tabs",
        "Dark theme",
        "Mobile History and Management",
      ],
      accessibility: results,
      screenshots: [
        "/tmp/suppervisor-desktop.png",
        "/tmp/suppervisor-mobile.png",
      ],
    },
    null,
    2,
  ),
);
