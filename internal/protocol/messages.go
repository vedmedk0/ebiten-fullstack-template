package protocol

import "encoding/json"

// MessageType discriminates protocol messages.
type MessageType string

const (
	// MsgWelcome is sent from server to the newly connected client with their
	// assigned ID and color.
	MsgWelcome MessageType = "welcome"

	// MsgJoin is broadcast by the server when a new player joins.
	MsgJoin MessageType = "join"

	// MsgLeave is broadcast by the server when a player disconnects.
	MsgLeave MessageType = "leave"

	// MsgPosition is sent from client to server with the player's current position.
	MsgPosition MessageType = "position"

	// MsgState is broadcast by the server with the full game state (all players).
	MsgState MessageType = "state"
)

// Envelope wraps every protocol message with a type discriminator.
type Envelope struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Color represents an RGB color.
type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

// WelcomeData is sent to a newly connected client.
type WelcomeData struct {
	ID    string `json:"id"`
	Color Color  `json:"color"`
}

// JoinData is broadcast when a new player joins.
type JoinData struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Color Color   `json:"color"`
}

// LeaveData is broadcast when a player disconnects.
type LeaveData struct {
	ID string `json:"id"`
}

// PositionData is sent by a client to update their position.
type PositionData struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PlayerInfo describes a single player inside a state snapshot.
type PlayerInfo struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Color Color   `json:"color"`
}

// StateData contains the complete game state broadcast to all clients.
type StateData struct {
	Players []PlayerInfo `json:"players"`
}

// Marshal encodes a typed protocol message into a JSON envelope.
func Marshal(msgType MessageType, data interface{}) ([]byte, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{Type: msgType, Data: raw})
}

// Unmarshal decodes a JSON envelope. Callers switch on env.Type and then
// json.Unmarshal env.Data into the appropriate *Data struct.
func Unmarshal(b []byte) (Envelope, error) {
	var env Envelope
	err := json.Unmarshal(b, &env)
	return env, err
}
