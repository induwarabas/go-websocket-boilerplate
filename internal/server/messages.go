package server

import (
	"encoding/json"
	"log/slog"
)

type IngressMsg struct {
	InMsgType string          `json:"type,omitempty"`
	InMsgCh   string          `json:"ch,omitempty"`
	InMsgID   string          `json:"id,omitempty"`
	InMsgData json.RawMessage `json:"data,omitempty"`
}

func (i IngressMsg) ID() string {
	return i.InMsgID
}

func (i IngressMsg) Type() string {
	return i.InMsgType
}

func (i IngressMsg) Channel() string {
	return i.InMsgCh
}

func (i IngressMsg) Data() json.RawMessage {
	return i.InMsgData
}

type EgressMsg struct {
	Type    string          `json:"type,omitempty"`
	Channel string          `json:"ch,omitempty"`
	ID      string          `json:"id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func NewEgressMsg(id string, outMsgType string, channel string, data any) *EgressMsg {
	dt, err := json.Marshal(data)
	if err != nil {
		slog.Info("error marshalling data", "error", err)
	}
	return &EgressMsg{ID: id, Type: outMsgType, Channel: channel, Data: dt}
}

type AuthMsg struct {
	AuthToken string `json:"authToken"`
}
