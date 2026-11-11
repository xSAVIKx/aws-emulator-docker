// Scenario "Key-value table" (DynamoDB): create a table, wait until it is ACTIVE,
// put two items, get one, query both, delete the table.

using Amazon.DynamoDBv2;
using Amazon.DynamoDBv2.Model;

namespace AwsEmulator.Examples;

public class KeyValueTableTests(EmulatorFixture aws)
{
    [Fact]
    public async Task Table_PutGetQuery()
    {
        var ct = TestContext.Current.CancellationToken;
        var table = EmulatorFixture.UniqueName();

        // A table with a composite key: partition key "pk" and sort key "sk".
        await aws.DynamoDb.CreateTableAsync(new CreateTableRequest
        {
            TableName = table,
            BillingMode = BillingMode.PAY_PER_REQUEST,
            AttributeDefinitions =
            [
                new AttributeDefinition("pk", ScalarAttributeType.S),
                new AttributeDefinition("sk", ScalarAttributeType.S),
            ],
            KeySchema =
            [
                new KeySchemaElement("pk", KeyType.HASH),
                new KeySchemaElement("sk", KeyType.RANGE),
            ],
        }, ct);
        try
        {
            // CreateTable returns CREATING. Poll until the table is ACTIVE.
            await WaitUntilActiveAsync(table, ct);

            foreach (var (sk, name) in new[] { ("1", "first"), ("2", "second") })
            {
                await aws.DynamoDb.PutItemAsync(new PutItemRequest
                {
                    TableName = table,
                    Item = new Dictionary<string, AttributeValue>
                    {
                        ["pk"] = new() { S = "user" },
                        ["sk"] = new() { S = sk },
                        ["name"] = new() { S = name },
                    },
                }, ct);
            }

            // Get one item by its full key.
            var get = await aws.DynamoDb.GetItemAsync(new GetItemRequest
            {
                TableName = table,
                Key = new Dictionary<string, AttributeValue>
                {
                    ["pk"] = new() { S = "user" },
                    ["sk"] = new() { S = "1" },
                },
                ConsistentRead = true,
            }, ct);
            Assert.Equal("first", get.Item["name"].S);

            // Query both items by the partition key. The result is sorted by "sk".
            var query = await aws.DynamoDb.QueryAsync(new QueryRequest
            {
                TableName = table,
                KeyConditionExpression = "pk = :pk",
                ExpressionAttributeValues = new Dictionary<string, AttributeValue>
                {
                    [":pk"] = new() { S = "user" },
                },
            }, ct);
            Assert.Equal(["first", "second"], query.Items.Select(i => i["name"].S));
        }
        finally
        {
            await aws.DynamoDb.DeleteTableAsync(new DeleteTableRequest { TableName = table }, CancellationToken.None);
        }
    }

    private async Task WaitUntilActiveAsync(string table, CancellationToken ct)
    {
        var deadline = DateTime.UtcNow.AddSeconds(30);
        while (true)
        {
            var describe = await aws.DynamoDb.DescribeTableAsync(new DescribeTableRequest { TableName = table }, ct);
            if (describe.Table.TableStatus == TableStatus.ACTIVE)
            {
                return;
            }

            if (DateTime.UtcNow > deadline)
            {
                throw new TimeoutException($"Table {table} is still {describe.Table.TableStatus} after 30 s.");
            }

            await Task.Delay(TimeSpan.FromMilliseconds(250), ct);
        }
    }
}
