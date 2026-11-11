package example;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sqs.model.QueueDoesNotExistException;

/** Queue scenario: create a queue, send a message, receive and delete it, delete the queue. */
@ExtendWith(Emulator.class)
class QueueTest {

    private static SqsClient sqs;
    private static String queueUrl;

    @BeforeAll
    static void createClient() {
        sqs = SqsClient.create();
    }

    @AfterAll
    static void cleanUp() {
        if (sqs == null) {
            return;
        }
        try {
            if (queueUrl != null) {
                sqs.deleteQueue(b -> b.queueUrl(queueUrl));
            }
        } catch (QueueDoesNotExistException alreadyDeleted) {
            // The test deleted the queue. There is nothing to do.
        } finally {
            sqs.close();
        }
    }

    @Test
    void sendReceiveAndDeleteAMessage() {
        // The emulator returns a URL like https://sqs.us-east-1.amazonaws.com/000000000000/<name>.
        // Use it as an identifier and give it back unchanged. The SDK still sends the request
        // to AWS_ENDPOINT_URL.
        queueUrl = sqs.createQueue(b -> b.queueName(Emulator.uniqueName("queue"))).queueUrl();

        sqs.sendMessage(b -> b.queueUrl(queueUrl).messageBody("hello from Java"));

        // Long polling: the call waits up to 5 seconds for a message.
        List<Message> messages = sqs.receiveMessage(b -> b
                .queueUrl(queueUrl)
                .maxNumberOfMessages(1)
                .waitTimeSeconds(5)).messages();
        assertThat(messages).extracting(Message::body).containsExactly("hello from Java");

        // A received message stays in the queue until you delete it with its receipt handle.
        sqs.deleteMessage(b -> b.queueUrl(queueUrl).receiptHandle(messages.get(0).receiptHandle()));

        sqs.deleteQueue(b -> b.queueUrl(queueUrl));
        queueUrl = null;
    }
}
