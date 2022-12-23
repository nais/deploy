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

const VerificationTeam = "nais-verification"

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
	Message string `json:"message,omitempty"`
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

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	encodedSignature := r.Header.Get(api_v1.SignatureHeader)
	signature, err := hex.DecodeString(encodedSignature)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response.Message = "HMAC digest must be hex encoded"
		response.render(w)
		logger.Errorf("unable to validate team: %s: %s", response.Message, err)
		return
	}

	logger.Tracef("Request has hex encoded data in signature header")

	request := &Request{}
	if err := json.Unmarshal(data, request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response.Message = fmt.Sprintf("unable to unmarshal request body: %s", err)
		response.render(w)
		logger.Error(response.Message)
		return
	}

	logger = logger.WithFields(request.LogFields())
	logger.Tracef("Request has valid JSON")

	err = request.validate()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response.Message = fmt.Sprintf("invalid provision request: %s", err)
		response.render(w)
		logger.Error(response.Message)
		return
	}

	logger.Tracef("Request body validated successfully")

	if !api_v1.ValidateMAC(data, signature, h.SecretKey) {
		w.WriteHeader(http.StatusForbidden)
		response.Message = api_v1.FailedAuthenticationMsg
		response.render(w)
		logger.Errorf("%s: HMAC signature error", api_v1.FailedAuthenticationMsg)
		return
	}

	logger.Tracef("HMAC signature validated successfully")

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
		w.WriteHeader(http.StatusNoContent)
		return
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

	if request.Team == VerificationTeam {
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(
			struct {
				ApiKey  api_v1.Key `json:"apiKey,omitempty"`
				Message string     `json:"message,omitempty"`
			}{
				ApiKey:  key,
				Message: "API key provisioned successfully",
			})
		if err != nil {
			logger.Errorf("encode response: %v", err)
			return
		}

		logger.Infof("API key provisioned successfully")
		return
	}

	w.WriteHeader(http.StatusCreated)
	response.Message = "API key provisioned successfully"
	response.render(w)
	logger.Infof(response.Message)
}
