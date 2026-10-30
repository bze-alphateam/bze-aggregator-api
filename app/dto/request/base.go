package request

type ErrResponse struct {
	Message string `json:"message"`
}

func NewErrResponse(message string) ErrResponse {
	return ErrResponse{Message: message}
}
