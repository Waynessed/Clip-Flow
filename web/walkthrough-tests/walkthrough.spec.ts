import { test, expect } from "@playwright/test";

test("recorded clips play and scenarios expose real evidence without mutation requests", async ({
  page,
}) => {
  const errors: string[] = [];
  const requests: { method: string; url: string }[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("request", (request) =>
    requests.push({ method: request.method(), url: request.url() }),
  );
  await page.goto("./");
  await expect(
    page.getByText("Read-only walkthrough", { exact: true }),
  ).toBeVisible();
  await expect(page.locator("input[type=file], form")).toHaveCount(0);
  const video = page.getByLabel("Processed preview", { exact: true });
  await expect
    .poll(() => video.evaluate((el: HTMLVideoElement) => el.videoHeight))
    .toBe(480);
  await video.evaluate(async (el: HTMLVideoElement) => {
    el.muted = true;
    await el.play();
  });
  await expect
    .poll(() => video.evaluate((el: HTMLVideoElement) => el.currentTime))
    .toBeGreaterThan(0.2);
  await video.evaluate((el: HTMLVideoElement) => el.pause());
  const metadataURL = await page
    .getByRole("link", { name: "View metadata", exact: true })
    .getAttribute("href");
  const metadata = await page.request.get(metadataURL!);
  expect(metadata.ok()).toBeTruthy();
  expect((await metadata.json()).preview).toMatchObject({
    width: 854,
    height: 480,
    size: 196753,
  });
  await page
    .getByRole("button", { name: "Original input", exact: true })
    .click();
  await expect
    .poll(() =>
      page
        .getByLabel("Original input", { exact: true })
        .evaluate((el: HTMLVideoElement) => el.videoHeight),
    )
    .toBe(540);
  await page
    .getByRole("button", { name: /Recovery after worker termination/ })
    .click();
  await expect(
    page.getByRole("heading", {
      name: "Recovery after worker termination",
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.locator(".attempts li")).toHaveCount(2);
  await expect(page.locator(".attempts")).toContainText("expired");
  await page
    .getByRole("button", { name: /An obsolete result was rejected/ })
    .click();
  await expect(page.locator(".rejection")).toContainText("stale_publication");
  await expect(
    page.getByRole("link", { name: "Read the recorded rejection log" }),
  ).toHaveAttribute("href", /d1434ab.*final-recovery-stale.log/);
  await page.getByRole("button", { name: /A bounded storage failure/ }).click();
  await expect(page.locator(".attempts li")).toHaveCount(3);
  await expect(page.locator("video")).toHaveCount(0);
  await expect(page.locator(".failure-panel")).toContainText(
    "No outputs were published",
  );
  await page.getByRole("button", { name: /5\s*Publish/ }).click();
  await expect(page.locator(".stage-detail")).toContainText(
    "Only the valid attempt can publish",
  );
  await page
    .getByText("Inspect benchmark results and limits", { exact: true })
    .click();
  await expect(page.locator("table")).toContainText("145.13");
  expect(requests.length).toBeGreaterThan(2);
  expect(requests.every((request) => request.method === "GET")).toBeTruthy();
  expect(
    requests.every((request) =>
      request.url.startsWith("http://127.0.0.1:4173/Clip-Flow/"),
    ),
  ).toBeTruthy();
  expect(
    requests.some((request) => /\/v1\/|localhost:8080/.test(request.url)),
  ).toBeFalsy();
  expect(errors).toEqual([]);
});

test("mobile walkthrough fits and keyboard navigation reaches scenario controls", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("./");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Skip to walkthrough" }),
  ).toBeFocused();
  await page.keyboard.press("Enter");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("button", { name: /A processed clip/ }),
  ).toBeFocused();
  await expect
    .poll(() =>
      page
        .getByLabel("Processed preview", { exact: true })
        .evaluate((el: HTMLVideoElement) => el.readyState),
    )
    .toBeGreaterThanOrEqual(2);
  await page.screenshot({
    path: "../.artifacts/walkthrough-mobile.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 1360, height: 1000 });
  await page.screenshot({
    path: "../.artifacts/walkthrough-desktop.png",
    fullPage: true,
  });
});
