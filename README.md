# Ebiten Fullstack Template

A Go-only fullstack multiplayer game template. The frontend uses **Ebiten** and compiles to **WebAssembly**; the backend uses **net/http** to serve the HTML page and **WebSockets** to talk to the client. Includes a "Hello World" demo: multiple players appear as colored dots and see each other move in real time.

## Tech Stack

- **Go** — single language for frontend and backend
- **Ebiten** v2 — 2D game library with WASM support
- **net/http** — standard library HTTP server
- **Makefile** — build orchestration

## Getting Started

```bash
make build    # builds both WASM client and server
make run      # starts the server on :8080
```

Open http://localhost:8080 in your browser (or multiple tabs) to see multiplayer dots.

**Controls:** Arrow keys, or click/hold (mouse) / touch and hold to move your dot toward the pointer. The client reconnects automatically if the connection drops. Stop the server with Ctrl+C for a graceful shutdown.
