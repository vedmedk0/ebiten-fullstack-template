package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/coder/websocket"
)

// playerCounter is a simple incrementing counter used to assign player IDs.
var playerCounter int

// nextPlayerID returns a unique player ID.
func nextPlayerID() string {
	playerCounter++
	return fmt.Sprintf("player-%d", playerCounter)
}

// Server holds the HTTP server components.
type Server struct {
	Hub  *Hub
	Addr string
	http *http.Server
}

// New creates a new Server on the given address.
func New(addr string) *Server {
	return &Server{
		Hub:  NewHub(),
		Addr: addr,
	}
}

// handleWebSocket upgrades the HTTP connection to a WebSocket and registers
// the new client with the hub.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow all origins for development
	})
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	remoteAddr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}
	log.Printf("websocket connected from %s", remoteAddr)

	id := nextPlayerID()
	client := NewClient(s.Hub, conn, id)
	s.Hub.register <- client

	// Start the read and write pumps in separate goroutines.
	go client.WritePump()
	go client.ReadPump()
}

// Run starts the hub and HTTP server.
func (s *Server) Run() error {
	go s.Hub.Run()

	mux := http.NewServeMux()

	// Serve static files from the web/ directory.
	fs := http.FileServer(http.Dir("web"))
	mux.Handle("/", fs)

	// WebSocket endpoint.
	mux.HandleFunc("/ws", s.handleWebSocket)

	s.http = &http.Server{
		Addr:    s.Addr,
		Handler: mux,
	}
	log.Printf("server listening on http://localhost%s", s.Addr)
	err := s.http.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// Shutdown gracefully shuts down the HTTP server and the hub.
func (s *Server) Shutdown(ctx context.Context) error {
	s.Hub.Stop()
	if s.http != nil {
		return s.http.Shutdown(ctx)
	}
	return nil
}
