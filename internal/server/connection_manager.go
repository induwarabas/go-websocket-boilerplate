package server

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Authenticator defines an interface for validating JWT tokens.
//
// The ValidateJwt method takes a JWT token string as input and returns the decoded claims
// or an error if the token is invalid.
type Authenticator interface {
	ValidateJwt(jwt string) (jwt.MapClaims, error)
}

// webSocketUpgrader configures the WebSocket upgrader with buffer sizes and a custom origin checker.
//
// CheckOrigin allows all incoming connections by returning true.
var webSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		// Allow all connections
		return true
	},
}

// ConnectionManager manages the active WebSocket clients, their connection handlers, and JWT authentication.
//
// It stores connected clients, handles new connections, and manages client disconnections.
type ConnectionManager struct {
	clients                 map[int]*WsClient       // Map of connected clients identified by an ID
	sync.RWMutex                                    // Mutex for safely handling client operations
	nextClientID            int                     // The ID for the next client connection
	clientConnectionHandler ClientConnectionHandler // Interface for handling client connection events
	authenticator           Authenticator           // Interface for validating client JWT tokens
}

// ClientConnectionHandler defines an interface for handling client connections.
//
// ClientConnected is called when a new WebSocket client successfully connects.
type ClientConnectionHandler interface {
	ClientConnected(client *WsClient)
}

// NewConnectionManager creates a new ConnectionManager with a client connection handler and authenticator.
//
// Params:
// - clientConnected: The handler responsible for managing connected clients.
// - authorize: The authenticator responsible for validating JWT tokens.
//
// Returns:
// - A pointer to the initialized ConnectionManager.
func NewConnectionManager(clientConnected ClientConnectionHandler, authorize Authenticator) *ConnectionManager {
	return &ConnectionManager{
		clients:                 make(map[int]*WsClient),
		nextClientID:            0,
		clientConnectionHandler: clientConnected,
		authenticator:           authorize,
	}
}

// addClient adds a WebSocket client to the connection manager's client list.
//
// Params:
// - client: A pointer to the WsClient that is being added.
func (m *ConnectionManager) addClient(client *WsClient) {
	m.Lock()
	defer m.Unlock()
	m.clients[client.ID()] = client
}

// removeClient removes a WebSocket client from the connection manager and closes the connection.
//
// Params:
// - client: A pointer to the WsClient that is being removed.
func (m *ConnectionManager) removeClient(client *WsClient) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client.ID()]; ok {
		client.Close()                 // Close the WebSocket connection
		delete(m.clients, client.ID()) // Remove the client from the list
	}
}

// ServeWs handles incoming WebSocket connection requests.
//
// It upgrades an HTTP connection to a WebSocket connection, validates the client's JWT token, and adds the client to the connection manager.
//
// Params:
// - w: The HTTP ResponseWriter used to send responses.
// - r: The HTTP request containing the connection details.
func (m *ConnectionManager) ServeWs(w http.ResponseWriter, r *http.Request) {
	m.nextClientID++
	log := slog.Default().With("conID", m.nextClientID) // Create a new logger with connection ID
	log.Info("New connection received.")
	authHeader := r.Header.Get("Authorization") // Retrieve the Authorization header
	var user jwt.MapClaims = nil                // Placeholder for the user's JWT claims
	var expire int64 = 0                        // Placeholder for the token expiration time

	// Validate the JWT token if Authorization header is present
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 {
			// JWT token is not properly formatted
			log.Info("Authorize failed.")
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("Authorize failed."))
			if err != nil {
				log.Info("Failed to write response", "error", err)
			}
			return
		}
		claims, err := m.authenticator.ValidateJwt(parts[1]) // Validate the token
		if err != nil {
			// Token validation failed
			log.Info("Authorize failed.")
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("Authorize failed."))
			if err != nil {
				log.Info("Failed to write response", "error", err)
			}
			return
		}
		user = claims // Store validated JWT claims
		exp, _ := claims.GetExpirationTime()
		expire = exp.Unix()
		log.Info("Authorize succeeded.", "expire", time.Unix(expire, 0).Format(time.RFC3339)) // Log token expiration time
	}

	// Create a new WebSocket client and upgrade the connection
	wsClient := NewClient(m.nextClientID, m, user, m.authenticator, expire)
	conn, err := webSocketUpgrader.Upgrade(w, r, nil) // Upgrade the connection to WebSocket
	if err != nil {
		// WebSocket upgrade failed
		log.Error("Websocket upgrade error", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("Websocket upgrade error"))
		if err != nil {
			log.Error("Failed to write response", "error", err)
		}
		return
	}

	// Set the WebSocket connection for the client and start handling messages
	wsClient.connection = conn
	m.addClient(wsClient)
	wsClient.Start() // Start handling WebSocket communication
}
