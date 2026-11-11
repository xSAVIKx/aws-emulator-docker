// Scenario: Key-value table (DynamoDB).
import {
  CreateTableCommand,
  DeleteTableCommand,
  DynamoDBClient,
  waitUntilTableExists,
} from "@aws-sdk/client-dynamodb";
import {
  DynamoDBDocumentClient,
  GetCommand,
  PutCommand,
  QueryCommand,
} from "@aws-sdk/lib-dynamodb";
import { afterAll, describe, expect, it } from "vitest";

import { uniqueName } from "./aws.js";

describe("key-value table (DynamoDB)", () => {
  const ddb = new DynamoDBClient({});
  // The DocumentClient converts plain JavaScript values to and from the
  // DynamoDB attribute format ({ S: "..." }, { N: "..." }).
  const doc = DynamoDBDocumentClient.from(ddb);
  const table = uniqueName();
  let created = false;

  afterAll(async () => {
    if (created) {
      await ddb.send(new DeleteTableCommand({ TableName: table })).catch(() => {});
    }
    doc.destroy();
    ddb.destroy();
  });

  it("creates a table, puts, gets and queries items, and deletes the table", async () => {
    // Partition key "pk" and sort key "sk", both strings.
    const createResult = await ddb.send(
      new CreateTableCommand({
        TableName: table,
        AttributeDefinitions: [
          { AttributeName: "pk", AttributeType: "S" },
          { AttributeName: "sk", AttributeType: "S" },
        ],
        KeySchema: [
          { AttributeName: "pk", KeyType: "HASH" },
          { AttributeName: "sk", KeyType: "RANGE" },
        ],
        BillingMode: "PAY_PER_REQUEST",
      }),
    );
    created = true;
    // Like AWS, the emulator returns CREATING first.
    expect(["CREATING", "ACTIVE"]).toContain(createResult.TableDescription?.TableStatus);

    // Poll DescribeTable until the table is ACTIVE.
    await waitUntilTableExists({ client: ddb, maxWaitTime: 20, minDelay: 1 }, { TableName: table });

    await doc.send(new PutCommand({ TableName: table, Item: { pk: "user#1", sk: "profile", name: "Ada" } }));
    await doc.send(new PutCommand({ TableName: table, Item: { pk: "user#1", sk: "settings", theme: "dark" } }));

    const got = await doc.send(
      new GetCommand({ TableName: table, Key: { pk: "user#1", sk: "profile" } }),
    );
    expect(got.Item).toEqual({ pk: "user#1", sk: "profile", name: "Ada" });

    const queried = await doc.send(
      new QueryCommand({
        TableName: table,
        KeyConditionExpression: "pk = :pk",
        ExpressionAttributeValues: { ":pk": "user#1" },
      }),
    );
    // Query returns the items in sort key order.
    expect(queried.Count).toBe(2);
    expect(queried.Items?.map((i) => i.sk)).toEqual(["profile", "settings"]);

    await ddb.send(new DeleteTableCommand({ TableName: table }));
    created = false;
  });
});
