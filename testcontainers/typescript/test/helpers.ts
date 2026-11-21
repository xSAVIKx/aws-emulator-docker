// Shared helpers for the tests.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

import { randomBytes } from "node:crypto";

import { NodeHttpHandler } from "@smithy/node-http-handler";

import type { StartedAwsEmulatorContainer } from "../src/index.js";

/**
 * The image under test. CI sets AWS_EMULATOR_IMAGE to test a local build.
 * Only the tests read this variable. The library does not.
 */
export const IMAGE = process.env.AWS_EMULATOR_IMAGE || "ghcr.io/xsavikx/aws-emulator:latest";

/** A resource name that is unique for this run, for example "tc-ts-3f9a1c2b". */
export function uniqueName(prefix = "tc-ts"): string {
  return `${prefix}-${randomBytes(4).toString("hex")}`;
}

/**
 * The common configuration for an SDK v3 client: endpoint, Region,
 * credentials, and an HTTPS handler that trusts only the emulator CA.
 */
export function clientConfig(container: StartedAwsEmulatorContainer) {
  return {
    ...container.getClientConfig(),
    requestHandler: new NodeHttpHandler({ httpsAgent: container.getHttpsAgent() }),
  };
}
