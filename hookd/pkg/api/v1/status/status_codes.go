package api_v1_status

import (
	"net/http"
)

var StatusCodes = []int{
	http.StatusOK,
	http.StatusNoContent,
	http.StatusBadRequest,
	http.StatusForbidden,
	http.StatusBadGateway,
	http.StatusInternalServerError,
}
