package ws

import (
	"sync"
)

// Room represents a channel that clients can join.
type Room struct {
	Name    string
	Client map[string]*Client
	mu     sync.RWMutex
}

// RoomManager manages multiple rooms.
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

// NewRoomManager creates a new room manager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
	}
}

// GetOrCreateRoom returns an existing room or creates a new one.
func (rm *RoomManager) GetOrCreateRoom(name string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[name]; exists {
		return room
	}

	room := &Room{
		Name:    name,
		Client: make(map[string]*Client),
	}
	rm.rooms[name] = room
	return room
}

// Join adds a client to a room.
func (rm *RoomManager) Join(roomName string, client *Client) {
	room := rm.GetOrCreateRoom(roomName)

	room.mu.Lock()
	room.Client[client.ID] = client
	room.mu.Unlock()
}

// Leave removes a client from a room.
func (rm *RoomManager) Leave(roomName string, client *Client) {
	rm.mu.RLock()
	room, exists := rm.rooms[roomName]
	rm.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.Lock()
	delete(room.Client, client.ID)
	clientCount := len(room.Client)
	room.mu.Unlock()

	// Clean up empty rooms
	if clientCount == 0 {
		rm.mu.Lock()
		delete(rm.rooms, roomName)
		rm.mu.Unlock()
	}
}

// LeaveAll removes a client from all rooms.
func (rm *RoomManager) LeaveAll(client *Client) {
	rm.mu.RLock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, room := range rm.rooms {
		rooms = append(rooms, room)
	}
	rm.mu.RUnlock()

	for _, room := range rooms {
		room.mu.Lock()
		delete(room.Client, client.ID)
		room.mu.Unlock()
	}
}

// BroadcastToRoom sends a message to all clients in a room.
func (rm *RoomManager) BroadcastToRoom(roomName string, msg []byte) {
	rm.mu.RLock()
	room, exists := rm.rooms[roomName]
	rm.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.RLock()
	defer room.mu.RUnlock()

	for _, client := range room.Client {
		select {
		case client.Send <- msg:
		default:
		}
	}
}

// RoomCount returns the number of clients in a room.
func (rm *RoomManager) RoomCount(roomName string) int {
	rm.mu.RLock()
	room, exists := rm.rooms[roomName]
	rm.mu.RUnlock()

	if !exists {
		return 0
	}

	room.mu.RLock()
	defer room.mu.RUnlock()
	return len(room.Client)
}