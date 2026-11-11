package example;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Map;

import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;

import software.amazon.awssdk.services.sns.SnsClient;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.Message;
import software.amazon.awssdk.services.sqs.model.QueueAttributeName;

/**
 * Fan-out scenario: create a topic and a queue, allow the topic in the queue policy, subscribe
 * with raw delivery, publish, receive the message.
 */
@ExtendWith(Emulator.class)
class FanOutTest {

    private static SnsClient sns;
    private static SqsClient sqs;
    private static String topicArn;
    private static String queueUrl;

    @BeforeAll
    static void createClients() {
        sns = SnsClient.create();
        sqs = SqsClient.create();
    }

    @AfterAll
    static void cleanUp() {
        // Delete in reverse order. Each step runs, also when an earlier step fails.
        try {
            if (topicArn != null) {
                sns.deleteTopic(b -> b.topicArn(topicArn)); // This also removes the subscription.
            }
        } finally {
            try {
                if (queueUrl != null) {
                    sqs.deleteQueue(b -> b.queueUrl(queueUrl));
                }
            } finally {
                sns.close();
                sqs.close();
            }
        }
    }

    @Test
    void topicDeliversARawMessageToTheQueue() {
        String name = Emulator.uniqueName("fanout");
        topicArn = sns.createTopic(b -> b.name(name)).topicArn();
        queueUrl = sqs.createQueue(b -> b.queueName(name)).queueUrl();
        String queueArn = sqs.getQueueAttributes(b -> b
                        .queueUrl(queueUrl)
                        .attributeNames(QueueAttributeName.QUEUE_ARN))
                .attributes().get(QueueAttributeName.QUEUE_ARN);

        // The queue policy lets SNS send messages to the queue, but only from this topic.
        String policy = """
                {
                  "Version": "2012-10-17",
                  "Statement": [{
                    "Effect": "Allow",
                    "Principal": {"Service": "sns.amazonaws.com"},
                    "Action": "sqs:SendMessage",
                    "Resource": "%s",
                    "Condition": {"ArnEquals": {"aws:SourceArn": "%s"}}
                  }]
                }""".formatted(queueArn, topicArn);
        sqs.setQueueAttributes(b -> b
                .queueUrl(queueUrl)
                .attributes(Map.of(QueueAttributeName.POLICY, policy)));

        // Raw delivery sends the message body as it is, without the SNS JSON envelope.
        sns.subscribe(b -> b
                .topicArn(topicArn)
                .protocol("sqs")
                .endpoint(queueArn)
                .attributes(Map.of("RawMessageDelivery", "true")));

        sns.publish(b -> b.topicArn(topicArn).message("order created"));

        // Delivery is asynchronous. Poll the queue for up to about 10 seconds.
        List<Message> messages = receiveWithin(Duration.ofSeconds(10));
        assertThat(messages).extracting(Message::body).containsExactly("order created");
    }

    /** Receives messages with long polling until one arrives or the time limit expires. */
    private static List<Message> receiveWithin(Duration limit) {
        Instant deadline = Instant.now().plus(limit);
        List<Message> messages;
        do {
            messages = sqs.receiveMessage(b -> b
                    .queueUrl(queueUrl)
                    .maxNumberOfMessages(1)
                    .waitTimeSeconds(2)).messages();
        } while (messages.isEmpty() && Instant.now().isBefore(deadline));
        return messages;
    }
}
