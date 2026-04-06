package kafka

import (
	"context"
	"errors"

	"github.com/segmentio/kafka-go"
)

// Consumer handles consuming messages from Kafka
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer creates a new Kafka consumer with consumer group
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        groupID,
			StartOffset:    kafka.LastOffset,
			CommitInterval: 0,    // auto-commit immediately
			MaxBytes:       10e6, // 10MB
		}),
	}
}

// Consume starts consuming messages and calls the handler for each message
func (c *Consumer) Consume(ctx context.Context, handler func([]byte) error) error {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		if err := handler(msg.Value); err != nil {
			// Offset not committed on error (will be retried)
			continue
		}
		// ReadMessage auto-commits on success
	}
}

// Close closes the reader
func (c *Consumer) Close() error {
	return c.reader.Close()
}
