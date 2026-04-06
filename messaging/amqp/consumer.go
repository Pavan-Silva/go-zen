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
		if cerr := conn.Close(); cerr != nil {
			return nil, fmt.Errorf("%w; close error: %v", err, cerr)
		}
		return nil, err
	}

	// Declare queue (idempotent)
	_, err = ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		if cerr := ch.Close(); cerr != nil {
			if rerr := conn.Close(); rerr != nil {
				return nil, fmt.Errorf("%w; channel close error: %v; conn close error: %v", err, cerr, rerr)
			}
			return nil, fmt.Errorf("%w; channel close error: %v", err, cerr)
		}
		if cerr := conn.Close(); cerr != nil {
			return nil, fmt.Errorf("%w; conn close error: %v", err, cerr)
		}
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
	var err error
	if c.channel != nil {
		if cerr := c.channel.Close(); cerr != nil {
			err = cerr
		}
	}
	if c.conn != nil {
		if cerr := c.conn.Close(); cerr != nil {
			if err != nil {
				err = fmt.Errorf("%v; conn close error: %w", err, cerr)
			} else {
				err = cerr
			}
		}
	}
	return err
}
