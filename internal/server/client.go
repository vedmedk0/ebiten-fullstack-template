package server

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"

	"ebiten-fullstack-template/internal/protocol"
)

const (
	writeWait      = 10 * time.Second
	maxMessageSize = 4096
	sendBufferSize = 256
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte

	// ID is the unique player identifier for this client.
	ID string
}

// NewClient creates a new Client.
func NewClient(hub *Hub, conn *websocket.Conn, id string) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, sendBufferSize),
		ID:   id,
	}
}

// ReadPump pumps messages from the websocket connection to the hub.
func (c *Client) ReadPump() {
	log.Printf("[%s] read pump started", c.ID)
	defer func() {
		log.Printf("[%s] read pump stopped", c.ID)
		c.hub.unregister <- c
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(maxMessageSize)
	ctx := context.Background()

	for {
		msgType, message, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure &&
				websocket.CloseStatus(err) != websocket.StatusGoingAway {
				log.Printf("websocket error: %v", err)
			}
			return
		}
		if msgType != websocket.MessageText {
			continue
		}

		env, err := protocol.Unmarshal(message)
		if err != nil {
			log.Printf("unmarshal error from %s: %v", c.ID, err)
			continue
		}

		switch env.Type {
		case protocol.MsgPosition:
			var pos protocol.PositionData
			if err := json.Unmarshal(env.Data, &pos); err != nil {
				log.Printf("unmarshal position error from %s: %v", c.ID, err)
				continue
			}
			c.hub.State.UpdatePosition(c.ID, pos.X, pos.Y)

			stateMsg := c.hub.buildStateMessage()
			c.hub.Broadcast(stateMsg)

		default:
			log.Printf("unknown message type from %s: %s", c.ID, env.Type)
		}
	}
}

// WritePump pumps messages from the hub to the websocket connection.
func (c *Client) WritePump() {
	log.Printf("[%s] write pump started", c.ID)
	defer func() {
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
		log.Printf("[%s] write pump stopped", c.ID)
	}()

	for message := range c.send {
		ctx, cancel := context.WithTimeout(context.Background(), writeWait)
		err := c.conn.Write(ctx, websocket.MessageText, message)
		cancel()
		if err != nil {
			return
		}
	}
}
