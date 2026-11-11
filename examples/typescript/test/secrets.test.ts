// Scenario: Secrets (Secrets Manager).
import {
  CreateSecretCommand,
  DeleteSecretCommand,
  GetSecretValueCommand,
  SecretsManagerClient,
} from "@aws-sdk/client-secrets-manager";
import { afterAll, describe, expect, it } from "vitest";

import { uniqueName } from "./aws.js";

describe("secrets (Secrets Manager)", () => {
  const secrets = new SecretsManagerClient({});
  const name = uniqueName();
  let created = false;

  afterAll(async () => {
    if (created) {
      await secrets
        .send(new DeleteSecretCommand({ SecretId: name, ForceDeleteWithoutRecovery: true }))
        .catch(() => {});
    }
    secrets.destroy();
  });

  it("creates a JSON secret, reads it back, and deletes it", async () => {
    const value = { username: "app", password: "s3cr3t-for-tests-only" };

    const createdSecret = await secrets.send(
      new CreateSecretCommand({ Name: name, SecretString: JSON.stringify(value) }),
    );
    created = true;
    expect(createdSecret.ARN).toContain(":000000000000:secret:");

    const read = await secrets.send(new GetSecretValueCommand({ SecretId: name }));
    expect(JSON.parse(read.SecretString ?? "null")).toEqual(value);

    // ForceDeleteWithoutRecovery deletes the secret now. Without it, the
    // secret waits 7 to 30 days for recovery, and its name stays in use.
    await secrets.send(new DeleteSecretCommand({ SecretId: name, ForceDeleteWithoutRecovery: true }));
    created = false;

    // The secret cannot be read any more. The error code can be
    // InvalidRequestException (marked for deletion) or ResourceNotFoundException.
    await expect(secrets.send(new GetSecretValueCommand({ SecretId: name }))).rejects.toThrow();
  });
});
