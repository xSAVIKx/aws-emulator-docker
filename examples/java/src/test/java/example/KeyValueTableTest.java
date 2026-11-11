package example;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.util.Map;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

import software.amazon.awssdk.core.retry.backoff.FixedDelayBackoffStrategy;
import software.amazon.awssdk.services.dynamodb.DynamoDbClient;
import software.amazon.awssdk.services.dynamodb.model.AttributeDefinition;
import software.amazon.awssdk.services.dynamodb.model.AttributeValue;
import software.amazon.awssdk.services.dynamodb.model.BillingMode;
import software.amazon.awssdk.services.dynamodb.model.KeySchemaElement;
import software.amazon.awssdk.services.dynamodb.model.KeyType;
import software.amazon.awssdk.services.dynamodb.model.QueryResponse;
import software.amazon.awssdk.services.dynamodb.model.ResourceNotFoundException;
import software.amazon.awssdk.services.dynamodb.model.ScalarAttributeType;
import software.amazon.awssdk.services.dynamodb.model.TableStatus;
import software.amazon.awssdk.services.dynamodb.waiters.DynamoDbWaiter;

/**
 * Key-value table scenario: create a table, wait until it is ACTIVE, put two items, get one,
 * query both, delete the table.
 */
@ExtendWith(Emulator.class)
class KeyValueTableTest {

    private static DynamoDbClient dynamo;
    private static final String TABLE = Emulator.uniqueName("table");
    /** True while the table exists. The clean-up step deletes the table only if it is true. */
    private static boolean tableExists;

    @BeforeAll
    static void createClient() {
        dynamo = DynamoDbClient.create();
    }

    @AfterAll
    static void cleanUp() {
        if (dynamo == null) {
            return;
        }
        try {
            if (tableExists) {
                dynamo.deleteTable(b -> b.tableName(TABLE));
            }
        } catch (ResourceNotFoundException alreadyDeleted) {
            // The table is gone. There is nothing to do.
        } finally {
            dynamo.close();
        }
    }

    /**
     * Makes a waiter that polls every second, for up to 60 seconds. The default waiter polls
     * every 20 seconds, which is slow for a local emulator.
     */
    private static DynamoDbWaiter fastWaiter() {
        return DynamoDbWaiter.builder()
                .client(dynamo)
                .overrideConfiguration(o -> o
                        .backoffStrategy(FixedDelayBackoffStrategy.create(Duration.ofSeconds(1)))
                        .maxAttempts(60))
                .build();
    }

    /** Makes a string attribute value. */
    private static AttributeValue s(String value) {
        return AttributeValue.fromS(value);
    }

    @Test
    void putGetAndQueryItems() {
        // The table has a composite key: partition key "pk" and sort key "sk".
        dynamo.createTable(b -> b
                .tableName(TABLE)
                .attributeDefinitions(
                        AttributeDefinition.builder().attributeName("pk").attributeType(ScalarAttributeType.S).build(),
                        AttributeDefinition.builder().attributeName("sk").attributeType(ScalarAttributeType.S).build())
                .keySchema(
                        KeySchemaElement.builder().attributeName("pk").keyType(KeyType.HASH).build(),
                        KeySchemaElement.builder().attributeName("sk").keyType(KeyType.RANGE).build())
                .billingMode(BillingMode.PAY_PER_REQUEST));
        tableExists = true;

        // CreateTable returns status CREATING. The waiter polls DescribeTable until it is ACTIVE.
        try (DynamoDbWaiter waiter = fastWaiter()) {
            waiter.waitUntilTableExists(b -> b.tableName(TABLE));
        }
        assertThat(dynamo.describeTable(b -> b.tableName(TABLE)).table().tableStatus())
                .isEqualTo(TableStatus.ACTIVE);

        dynamo.putItem(b -> b.tableName(TABLE).item(Map.of(
                "pk", s("user#1"), "sk", s("order#1"), "total", AttributeValue.fromN("10"))));
        dynamo.putItem(b -> b.tableName(TABLE).item(Map.of(
                "pk", s("user#1"), "sk", s("order#2"), "total", AttributeValue.fromN("25"))));

        Map<String, AttributeValue> item = dynamo.getItem(b -> b
                .tableName(TABLE)
                .key(Map.of("pk", s("user#1"), "sk", s("order#2")))
                .consistentRead(true)).item();
        assertThat(item.get("total").n()).isEqualTo("25");

        // A query reads all items with the same partition key, in sort key order.
        QueryResponse query = dynamo.query(b -> b
                .tableName(TABLE)
                .keyConditionExpression("pk = :pk")
                .expressionAttributeValues(Map.of(":pk", s("user#1"))));
        assertThat(query.items())
                .extracting(row -> row.get("sk").s())
                .containsExactly("order#1", "order#2");

        // DeleteTable returns status DELETING. A second DeleteTable call at this time fails with
        // ResourceInUseException, so wait until the table is gone.
        dynamo.deleteTable(b -> b.tableName(TABLE));
        try (DynamoDbWaiter waiter = fastWaiter()) {
            waiter.waitUntilTableNotExists(b -> b.tableName(TABLE));
        }
        tableExists = false;
    }
}
