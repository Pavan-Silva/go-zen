package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Pavan-Silva/go-zen"
	"github.com/Pavan-Silva/go-zen/auth"
	"github.com/Pavan-Silva/go-zen/logger"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// AllowedOrigins for WebSocket connections.
var AllowedOrigins []string

// SetAllowedOrigins configures which origins are allowed.
func SetAllowedOrigins(origins []string) {
	AllowedOrigins = origins
}

// WithOriginCheck configures custom origin check.
func WithOriginCheck(fn func(*http.Request) bool) {
	upgrader.CheckOrigin = fn
}

// Server holds the WebSocket infrastructure.
type Server struct {
	Hub     *Hub
	Rooms   *RoomManager
}

// NewServer creates a new WebSocket server.
func NewServer() *Server {
	return &Server{
		Hub:   NewHub(),
		Rooms: NewRoomManager(),
	}
}

// Run starts the WebSocket server.
func (s *Server) Run() {
	go s.Hub.Run()
}

// Shutdown stops the WebSocket server.
func (s *Server) Shutdown() {
	s.Hub.Shutdown()
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	return s.Hub.ClientCount()
}

// Handle handles WebSocket connections.
func (s *Server) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("ws: upgrade failed: %v", err)
		return
	}

	clientID := uuid.New().String()
	client := &Client{
		ID:   clientID,
		Conn: conn,
		Hub:  s.Hub,
		Send: make(chan []byte, 256),
	}

	s.Hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}

// HandleWithAuth handles authenticated WebSocket connections.
func (s *Server) HandleWithAuth(w http.ResponseWriter, r *http.Request, authenticator auth.Authenticator) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("ws: upgrade failed: %v", err)
		return
	}

	if _, err = authenticator.Authenticate(r); err != nil {
		logger.Error("ws auth failed: %v", err)
		conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unauthorized"),
			time.Now().Add(writeWait),
		)
		conn.Close()
		return
	}

	clientID := uuid.New().String()
	client := &Client{
		ID:    clientID,
		Conn:  conn,
		Hub:   s.Hub,
		Rooms: s.Rooms,
		Send:  make(chan []byte, 256),
	}

	s.Hub.register <- client

	go client.WritePump()
	go client.ReadPump()
}

// Message types for room operations.
type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

// HandleMessage processes incoming WebSocket messages.
func (s *Server) HandleMessage(client *Client, data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "join":
		s.Rooms.Join(msg.Room, client)
		response, _ := json.Marshal(Message{Type: "joined", Room: msg.Room})
		client.Send <- response

	case "leave":
		s.Rooms.Leave(msg.Room, client)
		response, _ := json.Marshal(Message{Type: "left", Room: msg.Room})
		client.Send <- response

	case "message":
		if msg.Room != "" {
			s.Rooms.BroadcastToRoom(msg.Room, data)
		} else {
			s.Hub.broadcast <- data
		}
	}
}

// Register registers a WebSocket route on the router.
func (s *Server) Register(r *zen.Router, pattern string) {
	r.HandleRaw(pattern, http.HandlerFunc(s.Handle))
}

// RegisterWithAuth registers an authenticated WebSocket route.
func (s *Server) RegisterWithAuth(r *zen.Router, pattern string, authenticator auth.Authenticator) {
	r.HandleRaw(pattern, http.HandlerFunc(func(w http.ResponseWriter, wq *http.Request) {
		s.HandleWithAuth(w, wq, authenticator)
	}))
}