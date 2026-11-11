// Scenario: Identity (STS).
import { GetCallerIdentityCommand, STSClient } from "@aws-sdk/client-sts";
import { afterAll, describe, expect, it } from "vitest";

describe("identity (STS)", () => {
  const sts = new STSClient({});

  afterAll(() => sts.destroy());

  it("GetCallerIdentity returns the emulator account", async () => {
    const identity = await sts.send(new GetCallerIdentityCommand({}));

    // The emulator uses the fixed account 000000000000.
    expect(identity.Account).toBe("000000000000");
    expect(identity.Arn).toContain("000000000000");
  });
});
