package msgs

const MsgAuthRequest = "AuthRequest"

type AuthRequest struct {
}

func NewAuthRequest() *AuthRequest {
	return &AuthRequest{}
}

func (p *AuthRequest) GetMsgType() MsgType {
	return MsgAuthRequest
}
