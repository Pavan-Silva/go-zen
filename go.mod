module github.com/Pavan-Silva/go-zen

go 1.25.0

require (
	// Core validation
	github.com/go-playground/validator/v10 v10.30.2

	// Authentication
	github.com/golang-jwt/jwt/v5 v5.3.1

	// Messaging - AMQP (RabbitMQ)
	github.com/rabbitmq/amqp091-go v1.10.0

	// Messaging - Kafka
	github.com/segmentio/kafka-go v0.4.50

	// WebSocket & gRPC
	golang.org/x/net v0.52.0
	google.golang.org/grpc v1.80.0
)

require (
	// Indirect dependencies
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
