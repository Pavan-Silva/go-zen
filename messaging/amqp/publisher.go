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
		if cerr := conn.Close(); cerr != nil {
			return nil, fmt.Errorf("%w; close error: %v", err, cerr)
		}
		return nil, err
	}

	// Enable publisher confirmations for reliability
	if err := ch.Confirm(false); err != nil {
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
	var err error
	if p.channel != nil {
		if cerr := p.channel.Close(); cerr != nil {
			err = cerr
		}
	}
	if p.conn != nil {
		if cerr := p.conn.Close(); cerr != nil {
			if err != nil {
				err = fmt.Errorf("%v; conn close error: %w", err, cerr)
			} else {
				err = cerr
			}
		}
	}
	return err
}
