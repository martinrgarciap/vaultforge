import { fileURLToPath } from "node:url";

import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const apiTarget =
  process.env.VAULTFORGE_API_TARGET?.trim() || "http://127.0.0.1:8080";

const webRoot = fileURLToPath(new URL(".", import.meta.url));
const cryptoWasmPackageRoot = fileURLToPath(
  new URL("../../packages/crypto-wasm/pkg", import.meta.url),
);

export default defineConfig({
  plugins: [react()],
  server: {
    fs: {
      allow: [webRoot, cryptoWasmPackageRoot],
    },
    proxy: {
      "/health": {
        target: apiTarget,
      },
      "/v1": {
        target: apiTarget,
      },
    },
  },
});
