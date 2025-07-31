package response

type ResponseData struct {
	Ec    int            `json:"ec"`
	Msg   string         `json:"msg,omitempty"`
	Error *ErrorResponse `json:"error,omitempty"`
	Total *int           `json:"total,omitempty"`
	Data  any            `json:"data,omitempty"`
}

type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
