import { defineConfig } from "astro/config";

export default defineConfig({
  site: "https://noalarms.dev",
  output: "static",
  build: { format: "file" },
});
