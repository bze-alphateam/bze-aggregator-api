package request

type ErrResponse struct {
	Message string `json:"message"`
}

func NewErrResponse(message string) ErrResponse {
	return ErrResponse{Message: message}
}

func NewUnknownErrorResponse() ErrResponse {
	return ErrResponse{Message: "Unknown error! Please report this issue to the administrators at alphateam@getbze.com"}
}
