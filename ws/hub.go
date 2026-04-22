package ws

import (
	"context"
	"sync"

	"github.com/Pavan-Silva/go-zen/logger"
)

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients    map[string]*Client
	mu         sync.RWMutex
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewHub creates and initializes a new Hub.
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:    make(map[string]*Client),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run starts the hub's main event loop.
func (h *Hub) Run() {
	for {
		select {
		case <-h.ctx.Done():
			h.mu.Lock()
			for _, client := range h.clients {
				close(client.Send)
			}
			h.clients = make(map[string]*Client)
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
			}
			h.mu.Unlock()
			logger.Info("ws: client disconnected: %s", client.ID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.Send <- msg:
				default:
					go func(c *Client) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Shutdown gracefully stops the hub.
func (h *Hub) Shutdown() {
	h.cancel()
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}