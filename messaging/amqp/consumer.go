package amqp

import (
	"context"
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
		conn.Close()
		return nil, err
	}

	// Declare queue (idempotent)
	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &Consumer{conn: conn, channel: ch, queue: queueName}, nil
}

// Consume starts consuming messages and calls the handler for each message
func (c *Consumer) Consume(ctx context.Context, handler func([]byte) error) error {
	msgs, err := c.channel.Consume(
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
		case delivery := <-msgs:
			if delivery.DeliveryTag == 0 {
				continue // channel closed
			}

			if err := handler(delivery.Body); err != nil {
				// Nack and requeue on error
				delivery.Nack(false, true)
				continue
			}

			// Ack on success
			delivery.Ack(false)
		}
	}
}

// Close closes the connection and channel
func (c *Consumer) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}