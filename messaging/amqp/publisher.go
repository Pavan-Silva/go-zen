package amqp

import (
	"context"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher handles publishing messages to RabbitMQ
type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewPublisher creates a new AMQP publisher
func NewPublisher(url string) (*Publisher, error) {
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

	// Enable publisher confirmations for reliability
	if err := ch.Confirm(false); err != nil {
		if closeErr := closeResources(ch, conn); closeErr != nil {
			return nil, fmt.Errorf("failed to enable publisher confirms: %w; cleanup error: %v", err, closeErr)
		}
		return nil, fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	return &Publisher{conn: conn, channel: ch}, nil
}

// Publish sends a message to the specified exchange with routing key
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return p.channel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // survive broker restart
		},
	)
}

// Close closes the connection and channel
func (p *Publisher) Close() error {
	return closeResources(p.channel, p.conn)
}

// closeResources closes channel and connection, combining any errors
func closeResources(channel *amqp.Channel, conn *amqp.Connection) error {
	var errs []error

	if channel != nil {
		if err := channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("channel close error: %w", err))
		}
	}

	if conn != nil {
		if err := conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("connection close error: %w", err))
		}
	}

	if len(errs) == 0 {
		return nil
	}

	if len(errs) == 1 {
		return errs[0]
	}

	return fmt.Errorf("%v", errs)
}
