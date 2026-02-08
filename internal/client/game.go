package client

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"ebiten-fullstack-template/internal/protocol"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480
	PlayerRadius = 8
	PlayerSpeed  = 3
)

// Game implements the ebiten.Game interface.
type Game struct {
	x, y    float64
	network *Network

	// Other players received from the server, keyed by player ID.
	players map[string]protocol.PlayerInfo

	// positionSynced is set once the client adopts the server-assigned spawn position.
	positionSynced bool

	// wasConnected tracks previous frame connection state to detect reconnect and reset positionSynced.
	wasConnected bool
}

// moveToward returns (nx, ny) one step of speed toward (tx, ty) from (cx, cy).
func moveToward(cx, cy, tx, ty, speed float64) (float64, float64) {
	dx, dy := tx-cx, ty-cy
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= speed {
		return tx, ty
	}
	return cx + dx/dist*speed, cy + dy/dist*speed
}

// NewGame creates a new Game with the player centered on the screen.
// If running inside a WASM environment, a WebSocket connection is
// established automatically.
func NewGame() *Game {
	return &Game{
		x:       float64(ScreenWidth) / 2,
		y:       float64(ScreenHeight) / 2,
		network: connectNetwork(),
		players: make(map[string]protocol.PlayerInfo),
	}
}

// Update handles input, sends position updates, and processes server messages.
func (g *Game) Update() error {
	prevX, prevY := g.x, g.y

	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		g.y -= PlayerSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		g.y += PlayerSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.x -= PlayerSpeed
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.x += PlayerSpeed
	}

	// Mouse: move toward cursor while left button is held.
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		cx, cy := ebiten.CursorPosition()
		g.x, g.y = moveToward(g.x, g.y, float64(cx), float64(cy), PlayerSpeed)
	}
	// Touch: move toward first touch position.
	for _, id := range ebiten.TouchIDs() {
		tx, ty := ebiten.TouchPosition(id)
		g.x, g.y = moveToward(g.x, g.y, float64(tx), float64(ty), PlayerSpeed)
		break
	}

	// Clamp to screen bounds.
	if g.x < PlayerRadius {
		g.x = PlayerRadius
	}
	if g.x > ScreenWidth-PlayerRadius {
		g.x = ScreenWidth - PlayerRadius
	}
	if g.y < PlayerRadius {
		g.y = PlayerRadius
	}
	if g.y > ScreenHeight-PlayerRadius {
		g.y = ScreenHeight - PlayerRadius
	}

	// Network: send position update when the player moved.
	if g.network != nil && g.network.IsConnected() && (g.x != prevX || g.y != prevY) {
		g.network.SendPosition(g.x, g.y)
	}

	// Network: process incoming server messages.
	if g.network != nil {
		connected := g.network.IsConnected()
		if connected && !g.wasConnected {
			g.positionSynced = false
		}
		g.wasConnected = connected
		g.processMessages()
	}

	return nil
}

// processMessages drains the network message queue and updates local state.
func (g *Game) processMessages() {
	for _, env := range g.network.ReceiveMessages() {
		switch env.Type {
		case protocol.MsgState:
			var state protocol.StateData
			if err := json.Unmarshal(env.Data, &state); err != nil {
				log.Printf("unmarshal state error: %v", err)
				continue
			}
			newPlayers := make(map[string]protocol.PlayerInfo, len(state.Players))
			for _, p := range state.Players {
				newPlayers[p.ID] = p
			}
			g.players = newPlayers

			// Sync local position from server on first state (randomized spawn).
			if !g.positionSynced && g.network != nil {
				if myID := g.network.PlayerID(); myID != "" {
					if me, ok := newPlayers[myID]; ok {
						g.x = me.X
						g.y = me.Y
						g.positionSynced = true
					}
				}
			}

		case protocol.MsgJoin:
			var join protocol.JoinData
			if err := json.Unmarshal(env.Data, &join); err != nil {
				log.Printf("unmarshal join error: %v", err)
				continue
			}
			g.players[join.ID] = protocol.PlayerInfo{
				ID: join.ID, X: join.X, Y: join.Y, Color: join.Color,
			}

		case protocol.MsgLeave:
			var leave protocol.LeaveData
			if err := json.Unmarshal(env.Data, &leave); err != nil {
				log.Printf("unmarshal leave error: %v", err)
				continue
			}
			delete(g.players, leave.ID)
		}
	}
}

// Draw renders the player dot, other players, and status text.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 34, G: 34, B: 34, A: 255})

	// Draw other players.
	myID := ""
	if g.network != nil {
		myID = g.network.PlayerID()
	}
	for _, p := range g.players {
		if p.ID == myID {
			continue // we draw ourselves below
		}
		vector.DrawFilledCircle(screen, float32(p.X), float32(p.Y), PlayerRadius,
			color.RGBA{R: p.Color.R, G: p.Color.G, B: p.Color.B, A: 255}, true)
	}

	// Draw the local player.
	playerColor := color.RGBA{R: 0, G: 200, B: 80, A: 255}
	if g.network != nil && g.network.PlayerID() != "" {
		c := g.network.PlayerColor()
		playerColor = color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
	}
	vector.DrawFilledCircle(screen, float32(g.x), float32(g.y), PlayerRadius, playerColor, true)
	// Draw a white outline ring so the local player is easy to identify.
	vector.StrokeCircle(screen, float32(g.x), float32(g.y), PlayerRadius+3, 1.5,
		color.RGBA{R: 255, G: 255, B: 255, A: 180}, true)

	// Status text.
	status := "Arrow keys / click / touch to move"
	if g.network != nil {
		if g.network.IsConnected() {
			count := len(g.players)
			status = fmt.Sprintf("%s | %d player(s) | Arrow keys / click / touch to move", g.network.PlayerID(), count)
		} else {
			status = "Connecting..."
		}
	}
	ebitenutil.DebugPrint(screen, status)
}

// Layout returns the logical screen size.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}
