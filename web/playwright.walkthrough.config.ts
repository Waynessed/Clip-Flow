import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./walkthrough-tests",
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:4173/Clip-Flow/",
    headless: true,
    trace: "retain-on-failure",
  },
  webServer: {
    command:
      "node node_modules/vite/bin/vite.js preview --config vite.walkthrough.config.ts",
    url: "http://127.0.0.1:4173/Clip-Flow/",
    reuseExistingServer: false,
  },
  reporter: [
    ["list"],
    ["json", { outputFile: "../.artifacts/walkthrough-playwright.json" }],
  ],
});
