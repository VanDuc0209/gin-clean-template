package constant

import (
	"net/http"

	"github.com/duccv/go-clean-template/internal/model/response"
)

var BAD_REQUEST = response.ResponseData{
	Ec:  http.StatusBadRequest,
	Msg: "Bad request",
}

var INVALID_REQUEST = response.ResponseData{
	Ec:  http.StatusBadRequest,
	Msg: "Invalid request payload",
}

var UNAUTHORIZED = response.ResponseData{
	Ec:  http.StatusUnauthorized,
	Msg: "Unauthorized",
}

var NOT_FOUND = response.ResponseData{
	Ec:  http.StatusNotFound,
	Msg: "Not found",
}

var TOKEN_EXPIRED = response.ResponseData{
	Ec:  419,
	Msg: "Token expired!",
}

var INTERNAL_SERVER_ERROR = response.ResponseData{
	Ec:  500,
	Msg: "Internal server error",
}

var FORBIDDEN = response.ResponseData{
	Ec:  403,
	Msg: "Forbidden",
}
