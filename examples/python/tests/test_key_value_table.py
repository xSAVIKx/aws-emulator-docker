"""Key-value table scenario: DynamoDB table, items, get and query."""


def test_create_table_put_get_query(dynamodb, unique_name):
    table = unique_name

    # A composite primary key: partition key "pk" and sort key "sk".
    dynamodb.create_table(
        TableName=table,
        AttributeDefinitions=[
            {"AttributeName": "pk", "AttributeType": "S"},
            {"AttributeName": "sk", "AttributeType": "S"},
        ],
        KeySchema=[
            {"AttributeName": "pk", "KeyType": "HASH"},
            {"AttributeName": "sk", "KeyType": "RANGE"},
        ],
        BillingMode="PAY_PER_REQUEST",
    )
    try:
        # CreateTable returns status CREATING. The waiter polls
        # DescribeTable until the status is ACTIVE.
        dynamodb.get_waiter("table_exists").wait(
            TableName=table, WaiterConfig={"Delay": 1, "MaxAttempts": 30}
        )
        assert dynamodb.describe_table(TableName=table)["Table"]["TableStatus"] == "ACTIVE"

        # The low-level client uses typed attribute values: {"S": ...} is a string.
        for sk, title in [("order#1", "Keyboard"), ("order#2", "Mouse")]:
            dynamodb.put_item(
                TableName=table,
                Item={"pk": {"S": "customer#42"}, "sk": {"S": sk}, "title": {"S": title}},
            )

        item = dynamodb.get_item(
            TableName=table,
            Key={"pk": {"S": "customer#42"}, "sk": {"S": "order#1"}},
            ConsistentRead=True,
        )["Item"]
        assert item["title"]["S"] == "Keyboard"

        result = dynamodb.query(
            TableName=table,
            KeyConditionExpression="pk = :pk",
            ExpressionAttributeValues={":pk": {"S": "customer#42"}},
            ConsistentRead=True,
        )
        assert result["Count"] == 2
        assert [i["sk"]["S"] for i in result["Items"]] == ["order#1", "order#2"]
    finally:
        dynamodb.delete_table(TableName=table)
