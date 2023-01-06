package api_v1_provision

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/nais/deploy/pkg/azure/graphapi"
	"github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/middleware"

	types "github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	APIKeyStorage database.ApiKeyStore
	TeamClient    graphapi.Client
	SecretKey     []byte
	GroupProvider middleware.GroupProvider
}

type Request struct {
	Team      string           `json:"team"`
	Rotate    bool             `json:"rotate"`
	Timestamp api_v1.Timestamp `json:"timestamp"`
}

type Response struct {
	Message string       `json:"message,omitempty"`
	ApiKeys []api_v1.Key `json:"apiKey,omitempty"`
}

func (r *Response) render(w io.Writer) {
	json.NewEncoder(w).Encode(r)
}

func (r *Request) validate() error {

	if len(r.Team) == 0 {
		return fmt.Errorf("no team specified")
	}

	if err := r.Timestamp.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *Request) LogFields() log.Fields {
	return log.Fields{
		types.LogFieldTeam: r.Team,
	}
}

func (h *Handler) ApiKey(w http.ResponseWriter, r *http.Request) {
	var err error
	var response Response

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	logger.Tracef("Incoming internal api key request")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response.Message = fmt.Sprintf("unable to read request body: %s", err)
		response.render(w)

		logger.Error(response.Message)
		return
	}

	request := h.validateRequest(w, r, logger, data)
	if request == nil {
		return
	}
	logger = logger.WithFields(request.LogFields())

	keys, err := h.APIKeyStorage.ApiKeys(r.Context(), request.Team)
	if err != nil {
		if database.IsErrNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			response.Message = "no api key found for team"
			response.render(w)
			logger.Infof("api key requested for team with no keys")
			return
		} else {
			w.WriteHeader(http.StatusBadGateway)
			response.Message = "unable to communicate with team API key backend"
			response.render(w)
			logger.Errorf("%s: %s", response.Message, err)
			return
		}
	}

	if len(keys.Valid()) != 0 {
		w.WriteHeader(http.StatusOK)
		response.ApiKeys = keys.ValidKeys()
		response.render(w)
		return
	} else {
		w.WriteHeader(http.StatusNotFound)
		response.Message = "no valid keys for team found"
		response.render(w)
		logger.Infof("no valid keys found for requested team")
		return
	}
}

func (h *Handler) ProvisionInternal(w http.ResponseWriter, r *http.Request) {
	h.provisionTeam(w, r, true)
}

func (h *Handler) ProvisionExternal(w http.ResponseWriter, r *http.Request) {
	h.provisionTeam(w, r, false)
}

func (h *Handler) provisionTeam(w http.ResponseWriter, r *http.Request, internalRequest bool) {
	var err error
	var response Response

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	logger.Tracef("Incoming provision request")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response.Message = fmt.Sprintf("unable to read request body: %s", err)
		response.render(w)

		logger.Error(response.Message)
		return
	}

	request := h.validateRequest(w, r, logger, data)
	if request == nil {
		return
	}
	logger = logger.WithFields(request.LogFields())

	keys, err := h.APIKeyStorage.ApiKeys(r.Context(), request.Team)
	if err != nil {
		if database.IsErrNotFound(err) {
			request.Rotate = true
		} else {
			w.WriteHeader(http.StatusBadGateway)
			response.Message = "unable to communicate with team API key backend"
			response.render(w)
			logger.Errorf("%s: %s", response.Message, err)
			return
		}
	}

	if !request.Rotate && len(keys.Valid()) != 0 {
		logger.Infof("Not overwriting existing team key which is still valid")
		if internalRequest {
			w.WriteHeader(http.StatusOK)
			response.Message = "team exists, returning existing keys"
			response.ApiKeys = keys.ValidKeys()
			response.render(w)
			return
		} else {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	key, err := api_v1.Keygen(api_v1.KeySize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response.Message = "unable to generate API key"
		response.render(w)
		logger.Errorf("%s: %s", response.Message, err)
		return
	}

	var teamId string
	switch h.GroupProvider {
	case middleware.GroupProviderAzure:
		azureTeam, err := h.TeamClient.Team(r.Context(), request.Team)
		if err != nil {
			if h.TeamClient.IsErrNotFound(err) {
				w.WriteHeader(http.StatusBadRequest)
				response.Message = "team does not exist in Azure AD"
				response.render(w)
				logger.Error(response.Message)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			response.Message = "unable to communicate with Azure AD"
			response.render(w)
			logger.Errorf("%s: %s", response.Message, err)
			return
		}
		teamId = azureTeam.AzureUUID
	case middleware.GroupProviderGoogle:
		teamId = request.Team
	default:
		w.WriteHeader(http.StatusInternalServerError)
		response.Message = "unable to resolve groups"
		response.render(w)
		logger.Errorf("no valid group provider selected")
		return
	}

	err = h.APIKeyStorage.RotateApiKey(r.Context(), request.Team, teamId, key)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		response.Message = "unable to persist API key"
		response.render(w)
		logger.Errorf("%s: %s", response.Message, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	response.Message = "API key provisioned successfully"
	if internalRequest {
		response.ApiKeys = []api_v1.Key{key}
	}
	response.render(w)
	logger.Infof(response.Message)
}

func (h *Handler) validateRequest(w http.ResponseWriter, r *http.Request, logger *log.Entry, data []byte) *Request {
	var response Response

	encodedSignature := r.Header.Get(api_v1.SignatureHeader)
	signature, err := hex.DecodeString(encodedSignature)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response.Message = "HMAC digest must be hex encoded"
		response.render(w)
		logger.Errorf("unable to validate team: %s: %s", response.Message, err)
		return nil
	}

	logger.Tracef("Request has hex encoded data in signature header")

	request := &Request{}
	if err := json.Unmarshal(data, request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response.Message = fmt.Sprintf("unable to unmarshal request body: %s", err)
		response.render(w)
		logger.Error(response.Message)
		return nil
	}

	logger = logger.WithFields(request.LogFields())
	logger.Tracef("Request has valid JSON")

	err = request.validate()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response.Message = fmt.Sprintf("invalid request: %s", err)
		response.render(w)
		logger.Error(response.Message)
		return nil
	}

	logger.Tracef("Request body validated successfully")

	if !api_v1.ValidateMAC(data, signature, h.SecretKey) {
		w.WriteHeader(http.StatusForbidden)
		response.Message = api_v1.FailedAuthenticationMsg
		response.render(w)
		logger.Errorf("%s: HMAC signature error", api_v1.FailedAuthenticationMsg)
		return nil
	}

	logger.Tracef("HMAC signature validated successfully")
	return request
}
