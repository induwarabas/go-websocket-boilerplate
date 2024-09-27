// Package server handles WebSocket connections and manages WebSocket clients.
package server

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"go-websocket-boilerplate/internal/handler"
	"log/slog"
	"time"
)

// Time allowed to read the next pong message from the client.
var pongWait = 10 * time.Second

// Send pings to client with this period. Must be less than pongWait.
var pingInterval = (pongWait * 9) / 10

// WsClient represents a WebSocket client, responsible for managing the connection,
// reading and writing messages, and handling authentication.
type WsClient struct {
	id            int                // Unique identifier for the client.
	manager       *ConnectionManager // Reference to the WebSocket connection manager.
	connection    *websocket.Conn    // WebSocket connection.
	ingress       chan handler.InMsg // Channel for incoming messages.
	egress        chan *EgressMsg    // Channel for outgoing messages.
	claims        jwt.MapClaims      // Claims associated with the client jwt token.
	context       context.Context    // Context to manage client lifecycle.
	cancel        context.CancelFunc // Cancel function to stop the client.
	expire        int64              // Authentication expiration time in Unix timestamp.
	authChannel   chan int64         // Channel for handling authentication expiration.
	authenticated bool               // Flag to indicate if the client is authenticated.
	authenticator Authenticator      // Authenticator for validating tokens.
	logger        *slog.Logger       // Logger for client specific logging
}

// Logger returns the logger associated with the client.
func (c *WsClient) Logger() *slog.Logger {
	return c.logger
}

// publishConnected sends a signal to the manager that the client has successfully connected.
func (c *WsClient) publishConnected() {
	c.manager.clientConnectionHandler.ClientConnected(c)
}

// SendResponse sends a response message to the client with the given details.
func (c *WsClient) SendResponse(id string, reqType string, channel string, data any) {
	c.egress <- NewEgressMsg(id, reqType, channel, data)
}

// SendUpdate sends an update message to the client.
func (c *WsClient) SendUpdate(updateType string, channel string, data any) {
	c.egress <- NewEgressMsg("", updateType, channel, data)
}

// Close closes the WebSocket connection for the client.
func (c *WsClient) Close() {
	c.cancel()
	if c.connection != nil {
		_ = c.connection.Close()
	}
}

// ID returns the client's unique identifier.
func (c *WsClient) ID() int {
	return c.id
}

// Context returns the context associated with the client.
func (c *WsClient) Context() context.Context {
	return c.context
}

// Ingress returns the channel for incoming messages.
func (c *WsClient) Ingress() chan handler.InMsg {
	return c.ingress
}

// Claims returns the claims associated with the client.
func (c *WsClient) Claims() jwt.MapClaims {
	return c.claims
}

// NewClient initializes and returns a new WebSocket client.
func NewClient(id int, manager *ConnectionManager, claims jwt.MapClaims, authenticator Authenticator, authExpire int64) *WsClient {
	ctx, cancelFunc := context.WithCancel(context.Background())
	expire := authExpire
	if expire == 0 {
		expire = time.Now().Add(30 * time.Second).Unix()
	}
	clientLogger := slog.Default().With("conID", id)
	if claims != nil {
		subject, _ := claims.GetSubject()
		clientLogger = clientLogger.With("sub", subject)
	} else {
		clientLogger = clientLogger.With("sub", "not_authenticated")
	}
	return &WsClient{
		manager:       manager,
		connection:    nil,
		egress:        make(chan *EgressMsg),
		ingress:       make(chan handler.InMsg),
		id:            id,
		context:       ctx,
		cancel:        cancelFunc,
		claims:        claims,
		authenticated: claims != nil,
		expire:        expire,
		authChannel:   make(chan int64),
		authenticator: authenticator,
		logger:        clientLogger,
	}
}

// readMessages reads and processes incoming WebSocket messages from the client.
func (c *WsClient) readMessages() {
	defer func() {
		c.manager.removeClient(c)
		c.cancel()
	}()

	// Set initial read deadline and limit message size.
	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait * 10)); err != nil {
		c.logger.Error("Error setting read deadline:", "error", err)
		return
	}
	c.connection.SetReadLimit(1024 * 1024) // 1MB

	// Set pong handler for ping/pong mechanism.
	c.connection.SetPongHandler(func(string) error {
		c.logger.Debug("pong")
		return c.connection.SetReadDeadline(time.Now().Add(pongWait * 10))
	})

	for {
		// Read the next message from the WebSocket connection.
		_, message, err := c.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("Websocket read error occurred", "error", err)
			}
			break
		}

		// Unmarshal the message into an IngressMsg.
		var request IngressMsg
		if err := json.Unmarshal(message, &request); err != nil {
			c.logger.Error("error unmarshalling event", "error", err)
			break
		}

		// Handle authentication messages.
		if request.Channel() == "sys" && request.Type() == "auth" {
			authMsg := &AuthMsg{}
			if err := json.Unmarshal(request.Data(), authMsg); err != nil {
				c.logger.Error("error unmarshalling auth msg", "error", err)
			} else {
				if authMsg.AuthToken == "" {
					c.logger.Error("invalid auth msg", "error", err)
				} else {
					claims, err := c.authenticator.ValidateJwt(authMsg.AuthToken)
					if err != nil {
						c.logger.Error("invalid auth msg", "error", err)
						c.Close()
						return
					}
					c.logger.Info("Successfully authenticated")
					if !c.authenticated {
						c.authenticated = true
						c.publishConnected()
					}
					c.claims = claims
					expirationTime, _ := claims.GetExpirationTime()
					c.logger.Info("Authorize succeeded.", "expire", time.Unix(expirationTime.Unix(), 0).Format(time.RFC3339))
					c.setAuthExpireTime(expirationTime.Unix())
				}
			}
		}

		// Pass the message to the ingress channel.
		c.ingress <- request
		c.logger.Debug("InMsg received")
	}
}

// writeMessages writes messages from the egress channel to the WebSocket connection.
func (c *WsClient) writeMessages() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		c.manager.removeClient(c)
		ticker.Stop()
	}()

	for {
		select {
		// Handle outgoing messages.
		case message, ok := <-c.egress:
			if !ok {
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					c.logger.Error("Error connection closed", "error", err)
				}
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				c.logger.Error("error marshalling event", "error", err)
			}
			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				c.logger.Error("Error sending message", "error", err)
			}
			c.logger.Debug("Message sent", "message", string(data))

		// Handle ping messages at regular intervals.
		case <-ticker.C:
			c.logger.Debug("Ping ticker...")
			if err := c.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Error("Error sending ping", "error", err)
				return
			}

		// Handle authentication expiration.
		case <-c.authChannel:
			c.logger.Info("Auth channel", "expire", c.expire, "now", time.Now().Unix(), "expireTime", time.Unix(c.expire, 0).Format(time.RFC3339), "nowTime", time.Now().Format(time.RFC3339))
			if c.expire <= time.Now().Unix() {
				c.logger.Error("Auth expire timeout")
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					c.logger.Error("Error connection closed", "error", err)
				}
			}

		// Stop the client when the context is done.
		case <-c.context.Done():
			return
		}
	}
}

// setAuthExpireTime sets the authentication expiration time and schedules an action after expiration.
func (c *WsClient) setAuthExpireTime(expire int64) {
	c.expire = expire
	time.AfterFunc(time.Unix(c.expire+1, 0).Sub(time.Now()), func() {
		c.authChannel <- c.expire
	})
}

// Start initializes the client's message reading and writing processes.
func (c *WsClient) Start() {
	go c.readMessages()
	go c.writeMessages()
	c.setAuthExpireTime(c.expire)
	if !c.authenticated {
		c.Logger().Info("Client not authenticated using bearer token. Waiting for auth message.")
	}
	if c.authenticated {
		c.publishConnected()
	}
}
