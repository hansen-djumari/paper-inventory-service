package types

type ApiResponse struct {
	Status int `json:"status"`
	Errors error `json:"errors"`
	Message string `json:"message"`
	Data any `json:"data"`
}
