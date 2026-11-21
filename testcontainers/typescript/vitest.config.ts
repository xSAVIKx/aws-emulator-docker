// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["test/**/*.test.ts"],
    // Run one file at a time: one emulator container at a time uses less
    // memory, and rootless Docker can fail when it maps two ports at once.
    fileParallelism: false,
    // A container start can take up to 120 s (the startup timeout).
    hookTimeout: 180_000,
    testTimeout: 60_000,
  },
});
