package server

import (
	"go-websocket-boilerplate/internal/handler"
	"log/slog"
	"net/http"
	"time"
)

// WsGw represents a WebSocket gateway that handles WebSocket server setup and authentication.
type WsGw struct {
	authenticator Authenticator // Interface for handling client authentication.
}

// NewWsGw creates a new instance of WsGw (WebSocket Gateway) with the provided Authenticator.
//
// Params:
// - authenticator: An interface that defines the authentication logic for WebSocket clients.
//
// Returns:
// - A pointer to the WsGw struct initialized with the given authenticator.
func NewWsGw(authenticator Authenticator) *WsGw {
	return &WsGw{authenticator: authenticator}
}

// Start initiates the WebSocket server.
//
// It sets up the connection manager, configures server timeouts, and listens on the /ws endpoint.
// The server logs information upon startup and handles errors if the server fails to start.
func (gw *WsGw) Start() {
	manager := NewConnectionManager(&DefaultClientConnectionHandler{}, gw.authenticator)

	// Configure the HTTP server with appropriate timeouts
	server := http.Server{
		Addr:              "localhost:3000", // Address to listen on
		ReadHeaderTimeout: 3 * time.Second,  // Time limit for reading headers
		ReadTimeout:       1 * time.Second,  // Time limit for reading the request body
		WriteTimeout:      1 * time.Second,  // Time limit for writing the response
		IdleTimeout:       30 * time.Second, // Maximum idle time for connections
	}
	http.HandleFunc("/ws", manager.ServeWs) // WebSocket connection handler

	// Log the server startup
	slog.Info("Server started on 0.0.0.0:3000")

	// Start the HTTP server and log errors if the server fails
	if err := server.ListenAndServe(); err != nil {
		slog.Error("ListenAndServe:", "error", err, "abs", "dilan")
	}
}

// DefaultClientConnectionHandler provides a default implementation for handling client connections.
//
// This implementation initializes a message handler for each connected client.
type DefaultClientConnectionHandler struct {
}

// ClientConnected is triggered when a new WebSocket client successfully connects.
//
// It initializes a message handler (`MsgHandler`) to manage client communication.
//
// Params:
// - client: A pointer to the WsClient representing the connected client.
func (d DefaultClientConnectionHandler) ClientConnected(client *WsClient) {
	clientHandler := handler.NewMsgHandler(client) // Create a new message handler
	clientHandler.Start()                          // Start handling messages
}
