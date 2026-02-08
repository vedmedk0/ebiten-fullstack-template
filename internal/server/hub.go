package server

import (
	"log"
	"math/rand"
	"sync"

	"ebiten-fullstack-template/internal/protocol"
)

// Predefined player colors for the demo.
var playerColors = []protocol.Color{
	{R: 231, G: 76, B: 60},   // red
	{R: 46, G: 204, B: 113},  // green
	{R: 52, G: 152, B: 219},  // blue
	{R: 155, G: 89, B: 182},  // purple
	{R: 241, G: 196, B: 15},  // yellow
	{R: 230, G: 126, B: 34},  // orange
	{R: 26, G: 188, B: 156},  // teal
	{R: 236, G: 240, B: 241}, // light grey
}

func randomColor() protocol.Color {
	return playerColors[rand.Intn(len(playerColors))]
}

// ----- Game State -----

// PlayerState holds a single player's position and color.
type PlayerState struct {
	X, Y  float64
	Color protocol.Color
}

// GameState tracks all connected players and their positions.
type GameState struct {
	mu      sync.RWMutex
	Players map[string]*PlayerState // keyed by player ID
}

// NewGameState creates an empty GameState.
func NewGameState() *GameState {
	return &GameState{
		Players: make(map[string]*PlayerState),
	}
}

// AddPlayer registers a new player.
func (gs *GameState) AddPlayer(id string, x, y float64, c protocol.Color) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.Players[id] = &PlayerState{X: x, Y: y, Color: c}
}

// RemovePlayer removes a player from the game state.
func (gs *GameState) RemovePlayer(id string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	delete(gs.Players, id)
}

// UpdatePosition updates a player's position.
func (gs *GameState) UpdatePosition(id string, x, y float64) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if p, ok := gs.Players[id]; ok {
		p.X = x
		p.Y = y
	}
}

// Snapshot returns a copy of all current player states.
func (gs *GameState) Snapshot() map[string]PlayerState {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	snap := make(map[string]PlayerState, len(gs.Players))
	for id, p := range gs.Players {
		snap[id] = *p
	}
	return snap
}

// ----- Hub -----

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from clients to broadcast.
	broadcast chan []byte

	// Register requests from clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Stop signals the hub to shut down and close all clients.
	stop chan struct{}

	// Game state tracking all players.
	State *GameState
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
		clients:    make(map[*Client]bool),
		State:      NewGameState(),
	}
}

// Stop shuts down the hub: closes all client send channels and exits the Run loop.
func (h *Hub) Stop() {
	close(h.stop)
}

// Run starts the hub's main event loop. It should be called in its own goroutine.
func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
				h.State.RemovePlayer(client.ID)
			}
			return
		case client := <-h.register:
			h.clients[client] = true

			// Assign a random color and a randomized starting position.
			c := randomColor()
			startX := 100.0 + rand.Float64()*440.0 // [100, 540]
			startY := 100.0 + rand.Float64()*280.0 // [100, 380]
			h.State.AddPlayer(client.ID, startX, startY, c)

			// Send welcome to the new client (their ID and color).
			if msg, err := protocol.Marshal(protocol.MsgWelcome, protocol.WelcomeData{
				ID: client.ID, Color: c,
			}); err == nil {
				client.send <- msg
			}

			// Send current game state so the new client sees existing players.
			client.send <- h.buildStateMessage()

			// Broadcast join to all clients.
			if msg, err := protocol.Marshal(protocol.MsgJoin, protocol.JoinData{
				ID: client.ID, X: startX, Y: startY, Color: c,
			}); err == nil {
				h.broadcastBytes(msg)
			}

			log.Printf("player joined: %s (%d total)", client.ID, len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.State.RemovePlayer(client.ID)

				// Broadcast leave to remaining clients.
				if msg, err := protocol.Marshal(protocol.MsgLeave, protocol.LeaveData{
					ID: client.ID,
				}); err == nil {
					h.broadcastBytes(msg)
				}

				log.Printf("player left: %s (%d total)", client.ID, len(h.clients))
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
					h.State.RemovePlayer(client.ID)
				}
			}
		}
	}
}

// Broadcast sends a raw message to all connected clients via the event loop.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

// broadcastBytes sends a raw message directly to every client's send channel.
// MUST be called only from the Hub.Run goroutine (which owns the clients map).
func (h *Hub) broadcastBytes(msg []byte) {
	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(h.clients, client)
			h.State.RemovePlayer(client.ID)
		}
	}
}

// buildStateMessage creates a MsgState envelope from the current game state.
func (h *Hub) buildStateMessage() []byte {
	snap := h.State.Snapshot()
	players := make([]protocol.PlayerInfo, 0, len(snap))
	for id, ps := range snap {
		players = append(players, protocol.PlayerInfo{
			ID: id, X: ps.X, Y: ps.Y, Color: ps.Color,
		})
	}
	msg, _ := protocol.Marshal(protocol.MsgState, protocol.StateData{Players: players})
	return msg
}
