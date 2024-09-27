package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"go-websocket-boilerplate/internal/msgs"
	"log/slog"
)

type InMsg interface {
	ID() string
	Type() string
	Channel() string
	Data() json.RawMessage
}

type Client interface {
	ID() int
	Context() context.Context
	SendResponse(id string, reqType string, channel string, data any)
	SendUpdate(updateType string, channel string, data any)
	Ingress() chan InMsg
	Close()
	Claims() jwt.MapClaims
	Logger() *slog.Logger
}

type MsgHandler struct {
	client Client
}

func NewMsgHandler(client Client) *MsgHandler {
	return &MsgHandler{
		client: client,
	}
}

func (m *MsgHandler) Start() {
	go m.listen()
	m.Logger().Info("msg handler started")
}

func (m *MsgHandler) listen() {
	for {
		select {
		case message := <-m.client.Ingress():
			m.onMessage(message)
		case <-m.client.Context().Done():
			m.Logger().Info("Client context done")
			return
		}
	}
}

func (m *MsgHandler) Logger() *slog.Logger {
	return m.client.Logger()
}

func (m *MsgHandler) onMessage(msg InMsg) {
	if msg.Channel() == "greeting" {
		m.HandleGreeting(msg)
	}
}

func (m *MsgHandler) HandleGreeting(msg InMsg) {
	greeting := &msgs.GreetingRequest{}
	err := json.Unmarshal(msg.Data(), greeting)
	if err != nil {
		m.client.SendResponse(msg.ID(), msg.Type(), msg.Channel(), &msgs.GreetingResponse{Message: "Invalid request"})
		return
	}
	validate := validator.New()
	err = validate.Struct(greeting)
	if err != nil {
		errorMsgs := make([]string, 0)
		for _, er := range err.(validator.ValidationErrors) {
			errorMsgs = append(errorMsgs, fmt.Sprintf("Field '%s' failed validation: %s\n", er.Field(), er.Tag()))
		}
		m.client.SendResponse(msg.ID(), msg.Type(), msg.Channel(), errorMsgs)
		return
	}
	m.client.SendResponse(msg.ID(), msg.Type(), msg.Channel(), &msgs.GreetingResponse{Message: fmt.Sprintf("Hello %s", greeting.Name)})
}
