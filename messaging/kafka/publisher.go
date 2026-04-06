package kafka

import (
	"context"
	"github.com/segmentio/kafka-go"
)

// Publisher handles publishing messages to Kafka
type Publisher struct {
	writer *kafka.Writer
}

// NewPublisher creates a new Kafka publisher
func NewPublisher(brokers []string, topic string) *Publisher {
	return &Publisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{}, // distribute load evenly
		},
	}
}

// Publish sends a message to Kafka with optional key
func (p *Publisher) Publish(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx,
		kafka.Message{
			Key:   key,
			Value: value,
		},
	)
}

// Close closes the writer
func (p *Publisher) Close() error {
	return p.writer.Close()
}