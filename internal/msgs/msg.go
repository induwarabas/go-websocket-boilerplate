package msgs

type MsgType string

type Msg interface {
	GetMsgType() MsgType
}
