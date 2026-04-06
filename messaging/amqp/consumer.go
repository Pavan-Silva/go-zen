package amqp

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer handles consuming messages from RabbitMQ
type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

// NewConsumer creates a new AMQP consumer
func NewConsumer(url, queueName string) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return nil, fmt.Errorf("%w; connection close error: %v", err, closeErr)
		}
		return nil, err
	}

	// Declare queue (idempotent)
	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		if closeErr := closeResources(ch, conn); closeErr != nil {
			return nil, fmt.Errorf("failed to declare queue: %w; cleanup error: %v", err, closeErr)
		}
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return &Consumer{conn: conn, channel: ch, queue: queueName}, nil
}

// Consume starts consuming messages and calls the handler for each message
func (c *Consumer) Consume(ctx context.Context, handler func([]byte) error) error {
	messages, err := c.channel.Consume(
		c.queue,
		"",    // consumer tag (auto-generated)
		false, // autoAck = false (manual acks)
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery := <-messages:
			if delivery.DeliveryTag == 0 {
				continue // channel closed
			}

			if err := handler(delivery.Body); err != nil {
				// Nack and requeue on error
				if nackErr := delivery.Nack(false, true); nackErr != nil {
					return fmt.Errorf("handler error: %w; nack error: %v", err, nackErr)
				}
				continue
			}

			// Ack on success
			if ackErr := delivery.Ack(false); ackErr != nil {
				return fmt.Errorf("ack error: %w", ackErr)
			}
		}
	}
}

// Close closes the connection and channel
func (c *Consumer) Close() error {
	return closeResources(c.channel, c.conn)
}
