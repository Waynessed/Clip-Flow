import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  root: "walkthrough",
  base: "/Clip-Flow/",
  publicDir: "../walkthrough-public",
  plugins: [react()],
  build: { outDir: "../dist-walkthrough", emptyOutDir: true },
  preview: { host: "127.0.0.1", port: 4173, strictPort: true },
});
