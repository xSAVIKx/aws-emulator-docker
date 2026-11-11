import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    // The guard runs once before any test file. It stops the run when the
    // endpoint is not local, so no test can reach a real AWS account.
    globalSetup: ["test/endpoint-guard.ts"],
    // Calls to the emulator are fast. The fan-out test waits up to ~10 s
    // for a message, so 30 s gives a large margin.
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
