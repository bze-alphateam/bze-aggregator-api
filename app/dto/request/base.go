package request

const (
	formatCoingecko = "coingecko"
)

type ErrResponse struct {
	Message string `json:"message"`
}

func NewErrResponse(message string) ErrResponse {
	return ErrResponse{Message: message}
}

func NewUnknownErrorResponse() ErrResponse {
	return ErrResponse{Message: "Unknown error! Please report this issue to the administrators at alphateam@getbze.com"}
}

// Format is the interface implemented by request parameters that allow returning the response in different formats
type Format interface {
	SetFormat(format string)
	GetFormat() string
}

func setAllowedFormat(f Format) {
	if f.GetFormat() != formatCoingecko {
		f.SetFormat("")
	}
}
