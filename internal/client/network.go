package client

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/coder/websocket"

	"ebiten-fullstack-template/internal/protocol"
)

const (
	reconnectInitial = 1 * time.Second
	reconnectMax     = 30 * time.Second
	defaultWSURL     = "ws://localhost:8080/ws"
)

// Network manages the WebSocket connection to the game server.
type Network struct {
	serverURL string
	messages  chan protocol.Envelope

	mu            sync.Mutex
	conn          *websocket.Conn
	connected     bool
	playerID      string
	playerColor   protocol.Color
	cancel        context.CancelFunc
	stopReconnect bool
}

// connectNetwork creates a Network and starts connecting to the server.
// It is called automatically by NewGame.
func connectNetwork() *Network {
	url := os.Getenv("WS_URL")
	if url == "" {
		url = defaultWSURL
	}
	n := &Network{
		serverURL: url,
		messages:  make(chan protocol.Envelope, 256),
	}
	go n.connectLoop()
	return n
}

func (n *Network) connectLoop() {
	delay := reconnectInitial
	for {
		n.mu.Lock()
		if n.stopReconnect {
			n.mu.Unlock()
			return
		}
		n.mu.Unlock()

		ctx, cancel := context.WithCancel(context.Background())
		n.mu.Lock()
		n.cancel = cancel
		n.mu.Unlock()

		log.Printf("connecting to %s", n.serverURL)
		conn, _, err := websocket.Dial(ctx, n.serverURL, nil)
		if err != nil {
			log.Printf("dial error: %v", err)
			cancel()
			time.Sleep(delay)
			if delay < reconnectMax {
				delay *= 2
				if delay > reconnectMax {
					delay = reconnectMax
				}
			}
			continue
		}

		n.mu.Lock()
		n.conn = conn
		n.connected = true
		n.playerID = ""
		n.playerColor = protocol.Color{}
		n.mu.Unlock()
		delay = reconnectInitial
		log.Println("websocket connected")

		n.readLoop(ctx, conn)

		n.mu.Lock()
		n.connected = false
		n.conn = nil
		n.mu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		cancel()
	}
}

func (n *Network) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure &&
				websocket.CloseStatus(err) != websocket.StatusGoingAway {
				log.Printf("read error: %v", err)
			}
			return
		}
		if msgType != websocket.MessageText {
			continue
		}

		var env protocol.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			log.Printf("unmarshal error: %v", err)
			continue
		}

		if env.Type == protocol.MsgWelcome {
			var w protocol.WelcomeData
			if err := json.Unmarshal(env.Data, &w); err == nil {
				n.mu.Lock()
				n.playerID = w.ID
				n.playerColor = w.Color
				n.mu.Unlock()
				log.Printf("welcome: id=%s color=(%d,%d,%d)",
					w.ID, w.Color.R, w.Color.G, w.Color.B)
			}
		}

		select {
		case n.messages <- env:
		default:
			log.Println("message channel full, dropping message")
		}
	}
}

// IsConnected reports whether the WebSocket is open.
func (n *Network) IsConnected() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.connected
}

// PlayerID returns the server-assigned player ID (empty until welcome received).
func (n *Network) PlayerID() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.playerID
}

// PlayerColor returns the server-assigned color.
func (n *Network) PlayerColor() protocol.Color {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.playerColor
}

// SendPosition sends a position update to the server.
func (n *Network) SendPosition(x, y float64) {
	n.mu.Lock()
	conn := n.conn
	connected := n.connected
	n.mu.Unlock()
	if !connected || conn == nil {
		return
	}
	data, err := protocol.Marshal(protocol.MsgPosition, protocol.PositionData{X: x, Y: y})
	if err != nil {
		log.Printf("marshal position error: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		log.Printf("write position error: %v", err)
	}
}

// ReceiveMessages drains all queued messages and returns them.
func (n *Network) ReceiveMessages() []protocol.Envelope {
	var msgs []protocol.Envelope
	for {
		select {
		case msg := <-n.messages:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// Close shuts down the WebSocket connection and stops reconnection.
func (n *Network) Close() {
	n.mu.Lock()
	n.stopReconnect = true
	if n.cancel != nil {
		n.cancel()
	}
	if n.conn != nil {
		_ = n.conn.Close(websocket.StatusNormalClosure, "")
		n.conn = nil
	}
	n.connected = false
	n.mu.Unlock()
}
