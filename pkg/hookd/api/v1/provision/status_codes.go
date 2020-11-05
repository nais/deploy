package api_v1_provision

import (
	"net/http"
)

var StatusCodes = []int{
	http.StatusCreated,
	http.StatusNoContent,
	http.StatusBadRequest,
	http.StatusForbidden,
	http.StatusBadGateway,
	http.StatusInternalServerError,
}
