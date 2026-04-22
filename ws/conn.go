package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait    = 10 * time.Second
	pongWait     = 60 * time.Second
	pingPeriod   = (pongWait * 9) / 10
	maxMsgSize   = 512 * 1024
)

// Client represents a single WebSocket connection.
type Client struct {
	ID    string
	Conn  *websocket.Conn
	Send  chan []byte
	Hub   *Hub
	Rooms *RoomManager
}

// ReadPump reads messages from the WebSocket connection.
func (c *Client) ReadPump() {
	defer func() {
		if c.Rooms != nil {
			c.Rooms.LeaveAll(c)
		}
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMsgSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
				websocket.CloseNormalClosure) {
			}
			break
		}

		// Handle room messages if room manager is available
		if c.Rooms != nil {
			c.handleMessage(msg)
		} else {
			c.Hub.broadcast <- msg
		}
	}
}

// handleMessage processes incoming WebSocket messages.
func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "join":
		c.Rooms.Join(msg.Room, c)
		response, _ := json.Marshal(Message{Type: "joined", Room: msg.Room})
		c.Send <- response

	case "leave":
		c.Rooms.Leave(msg.Room, c)
		response, _ := json.Marshal(Message{Type: "left", Room: msg.Room})
		c.Send <- response

	case "message":
		if msg.Room != "" {
			c.Rooms.BroadcastToRoom(msg.Room, data)
		} else {
			c.Hub.broadcast <- data
		}
	}
}

// WritePump writes messages to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(msg)

			n := len(c.Send)
			for range n {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}