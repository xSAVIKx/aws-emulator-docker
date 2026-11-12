package example

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

// Order is one item in the table. The dynamodbav tags name the attributes.
type Order struct {
	Customer string  `dynamodbav:"customer"` // partition key
	OrderID  string  `dynamodbav:"orderId"`  // sort key
	Total    float64 `dynamodbav:"total"`
}

// TestKeyValueTable creates a table, waits until it is ACTIVE, puts two items,
// gets one, queries both, and deletes the table.
func TestKeyValueTable(t *testing.T) {
	ctx := t.Context()
	client := dynamodb.NewFromConfig(loadConfig(t), func(o *dynamodb.Options) {
		// Real DynamoDB sends an X-Amz-Crc32 header with each response. The
		// emulator does not. Without this option, the SDK compares the body
		// with a checksum of 0 and logs a "failed to close HTTP response body"
		// warning for each call. The responses themselves are correct.
		o.DisableValidateResponseChecksum = true
	})
	table := uniqueName(t)

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(table),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("customer"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("orderId"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("customer"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("orderId"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	require.NoError(t, err)
	// Cleanup runs also when the test fails. It deletes the table only when
	// the test did not delete it.
	deleted := false
	t.Cleanup(func() {
		if !deleted {
			_, _ = client.DeleteTable(cleanupContext(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
		}
	})

	// CreateTable returns the status CREATING. The waiter polls DescribeTable
	// until the status is ACTIVE.
	waiter := dynamodb.NewTableExistsWaiter(client, func(o *dynamodb.TableExistsWaiterOptions) {
		o.MinDelay = 200 * time.Millisecond
		o.MaxDelay = 2 * time.Second
	})
	require.NoError(t, waiter.Wait(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)}, time.Minute))

	orders := []Order{
		{Customer: "alice", OrderID: "o-1", Total: 12.5},
		{Customer: "alice", OrderID: "o-2", Total: 30},
	}
	for _, o := range orders {
		// MarshalMap converts the struct into DynamoDB attribute values.
		item, err := attributevalue.MarshalMap(o)
		require.NoError(t, err)
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(table), Item: item})
		require.NoError(t, err)
	}

	key, err := attributevalue.MarshalMap(map[string]string{"customer": "alice", "orderId": "o-1"})
	require.NoError(t, err)
	got, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(table),
		Key:            key,
		ConsistentRead: aws.Bool(true),
	})
	require.NoError(t, err)
	var one Order
	require.NoError(t, attributevalue.UnmarshalMap(got.Item, &one))
	require.Equal(t, orders[0], one)

	query, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		KeyConditionExpression: aws.String("customer = :c"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":c": &types.AttributeValueMemberS{Value: "alice"},
		},
		ConsistentRead: aws.Bool(true),
	})
	require.NoError(t, err)
	var all []Order
	require.NoError(t, attributevalue.UnmarshalListOfMaps(query.Items, &all))
	// Query returns the items in sort key order.
	require.Equal(t, orders, all)

	_, err = client.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	require.NoError(t, err)
	deleted = true
}
