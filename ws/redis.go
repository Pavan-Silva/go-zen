package ws

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// RedisHub extends Hub with Redis pub/sub for horizontal scaling.
type RedisHub struct {
	*Hub
	client   *redis.Client
	channel  string
	instance string
}

// BroadcastMsg represents a message in Redis pub/sub.
type BroadcastMsg struct {
	Instance  string
	Room      string
	Data      []byte
	Timestamp int64
}

// NewRedisHub creates a hub with Redis pub/sub support.
func NewRedisHub(config RedisConfig, channel string) (*RedisHub, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()
	instance := hostname + "-" + time.Now().Format("20060102150405")

	rh := &RedisHub{
		Hub:      NewHub(),
		client:   client,
		channel:  channel,
		instance: instance,
	}

	log.Printf("ws: Redis hub initialized: instance=%s channel=%s", instance, channel)
	return rh, nil
}

// Run starts the hub and Redis subscription.
func (rh *RedisHub) Run() {
	go rh.Hub.Run()
	go rh.subscribe()
	go rh.publish()
}

// subscribe listens for messages from other instances.
func (rh *RedisHub) subscribe() {
	ctx := rh.Hub.ctx
	pubsub := rh.client.Subscribe(ctx, rh.channel)
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-ch:
			var bm BroadcastMsg
			if err := json.Unmarshal([]byte(msg.Payload), &bm); err != nil {
				continue
			}

			if bm.Instance == rh.instance {
				continue
			}

			rh.broadcastLocal(bm.Data)
		}
	}
}

// publish handles local broadcasts and publishes to Redis.
func (rh *RedisHub) publish() {
	for {
		select {
		case <-rh.Hub.ctx.Done():
			return

		case msg := <-rh.Hub.broadcast:
			rh.broadcastLocal(msg)
			rh.publishToRedis(msg, "")
		}
	}
}

// broadcastLocal sends a message to all local clients.
func (rh *RedisHub) broadcastLocal(msg []byte) {
	rh.Hub.mu.RLock()
	defer rh.Hub.mu.RUnlock()

	for _, client := range rh.Hub.clients {
		select {
		case client.Send <- msg:
		default:
		}
	}
}

// publishToRedis publishes a message to Redis.
func (rh *RedisHub) publishToRedis(data []byte, room string) {
	bm := BroadcastMsg{
		Instance:  rh.instance,
		Room:      room,
		Data:      data,
		Timestamp: time.Now().UnixNano(),
	}

	payload, err := json.Marshal(bm)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rh.client.Publish(ctx, rh.channel, payload)
}

// BroadcastToRoom publishes a message to a specific room.
func (rh *RedisHub) BroadcastToRoom(room string, data []byte) {
	rh.publishToRedis(data, room)
}

// Shutdown closes Redis connections.
func (rh *RedisHub) Shutdown() {
	rh.Hub.Shutdown()
	rh.client.Close()
}
