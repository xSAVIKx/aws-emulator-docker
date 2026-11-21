// A second container on the same network calls the emulator by its alias.
//
// Maintainer: Yurii Serhiichuk <yurii@serhiichuk.dev>
// SPDX-License-Identifier: Apache-2.0

import { GenericContainer, Network, type StartedNetwork, Wait } from "testcontainers";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { AwsEmulatorContainer, type StartedAwsEmulatorContainer } from "../src/index.js";
import { IMAGE } from "./helpers.js";

const CURL_IMAGE =
  "curlimages/curl:8.23.0@sha256:3fb7e13e5d179a845c97d78204f3f1064d1907037f26a3ebf91710571151fd93";

describe("network alias", () => {
  let network: StartedNetwork;
  let container: StartedAwsEmulatorContainer;

  beforeAll(async () => {
    network = await new Network().start();
    container = await new AwsEmulatorContainer(IMAGE).withNetwork(network).withNetworkAliases("aws").start();
  });

  afterAll(async () => {
    await container?.stop();
    await network?.stop();
  });

  it("a container on the network reaches https://aws:9297 with the CA", async () => {
    // The one-shot strategy waits until curl exits, and fails when the exit code is not 0.
    const curl = await new GenericContainer(CURL_IMAGE)
      .withNetwork(network)
      .withCopyContentToContainer([{ content: container.getCaCertificate(), target: "/ca.pem", mode: 0o644 }])
      .withCommand(["-sf", "--cacert", "/ca.pem", "https://aws:9297/__admin/health"])
      .withWaitStrategy(Wait.forOneShotStartup())
      .start();
    try {
      let output = "";
      for await (const chunk of await curl.logs()) output += chunk.toString();
      expect(output.trim()).toBe("ok");
    } finally {
      await curl.stop();
    }
  });
});
