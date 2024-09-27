package msgs

const MsgGreetingRequest = "GreetingRequest"

type GreetingRequest struct {
	Name string `json:"name" validate:"required"`
}

func (g *GreetingRequest) GetMsgType() MsgType {
	return MsgGreetingRequest
}

const MsgGreetingResponse = "GreetingResponse"

type GreetingResponse struct {
	Message string `json:"message"`
}

func (g *GreetingResponse) GetMsgType() MsgType {
	return MsgGreetingResponse
}
