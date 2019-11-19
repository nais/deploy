package api_v1_deploy

import (
	"net/http"
)

var StatusCodes = []int{
	http.StatusCreated,
	http.StatusBadRequest,
	http.StatusForbidden,
	http.StatusBadGateway,
	http.StatusInternalServerError,
}
