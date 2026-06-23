import { randomBytes } from "node:crypto";
import { defineConfig, devices } from "@playwright/test";

const apiOrigin = "http://127.0.0.1:8081";
const webOrigin = "http://127.0.0.1:4173";

const databaseURL =
  process.env.E2E_DATABASE_URL?.trim() ||
  "postgres://vaultforge:vaultforge_local_dev@127.0.0.1:5433/vaultforge_e2e?sslmode=disable";

const redisURL =
  process.env.E2E_REDIS_URL?.trim() || "redis://127.0.0.1:6380/2";

const testSigningSeed = Buffer.alloc(32, 0x42).toString("base64");

const testRateLimitIdentityKey = randomBytes(32).toString("base64");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 180_000,

  expect: {
    timeout: 10_000,
  },

  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,

  reporter: [
    ["list"],
    [
      "html",
      {
        open: "never",
      },
    ],
  ],

  use: {
    baseURL: webOrigin,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },

  projects: [
    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
      },
    },
  ],

  webServer: [
    {
      command: "cd ../api && go run ./cmd/api",
      url: `${apiOrigin}/health/ready`,
      timeout: 120_000,
      reuseExistingServer: false,
      env: {
        APP_ENV: "test",
        HTTP_ADDR: "127.0.0.1:8081",
        DATABASE_URL: databaseURL,
        REDIS_URL: redisURL,
        REDIS_DIAL_TIMEOUT: "2s",
        REDIS_READ_TIMEOUT: "1s",
        REDIS_WRITE_TIMEOUT: "1s",
        REDIS_POOL_TIMEOUT: "2s",
        RATE_LIMIT_IDENTITY_HMAC_KEY_BASE64: testRateLimitIdentityKey,
        ACCESS_TOKEN_ISSUER: "vaultforge-e2e",
        ACCESS_TOKEN_AUDIENCE: "vaultforge-e2e",
        ACCESS_TOKEN_KEY_ID: "playwright-ed25519-v1",
        ACCESS_TOKEN_TTL: "10m",
        REFRESH_TOKEN_TTL: "1h",
        ACCESS_TOKEN_CLOCK_LEEWAY: "30s",
        ACCESS_TOKEN_ED25519_SEED_BASE64: testSigningSeed,
      },
    },
    {
      command: "npm run dev -- --host 127.0.0.1 --port 4173 --strictPort",
      url: webOrigin,
      timeout: 120_000,
      reuseExistingServer: false,
      env: {
        VAULTFORGE_API_TARGET: apiOrigin,
      },
    },
  ],
});
