import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const apiTarget = "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  server: {
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
