package response

type ResponseData struct {
	Ec    int    `json:"ec"`
	Msg   string `json:"msg,omitempty"`
	Error string `json:"error,omitempty"`
	Total *int   `json:"total,omitempty"`
	Data  any    `json:"data,omitempty"`
}
