package auth_interceptor

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/metrics"
	"github.com/nais/deploy/pkg/pb"
)

const (
	requestTypeApiKey = "api_key"
	requestTypeJWT    = "jwt"
)

type ServerInterceptor struct {
	APIKeyStore    database.ApiKeyStore
	TokenValidator TokenValidator
	TeamsClient    TeamsClient
}

type TokenValidator interface {
	Validate(ctx context.Context, token string) (jwt.Token, error)
}

type TeamsClient interface {
	IsAuthorized(ctx context.Context, repo, team string) bool
}

type authData struct {
	hmac      []byte
	timestamp string
	team      string
}

func NewServerInterceptor(apiKeyStore database.ApiKeyStore, tokenValidator TokenValidator, teamsClient TeamsClient) *ServerInterceptor {
	return &ServerInterceptor{
		APIKeyStore:    apiKeyStore,
		TokenValidator: tokenValidator,
		TeamsClient:    teamsClient,
	}
}

func (s *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	_, ok := req.(*pb.DeploymentRequest)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "requests to this endpoint must be DeploymentRequest")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid metadata in request")
	}

	jwt := get("jwt", md)

	if jwt != "" {
		t, err := s.TokenValidator.Validate(ctx, jwt)
		if err != nil {
			metrics.InterceptorRequest(requestTypeJWT, "invalid_jwt")
			return nil, status.Errorf(codes.Unauthenticated, "invalid JWT token")
		}

		r, ok := t.Get("repository")
		if !ok {
			metrics.InterceptorRequest(requestTypeJWT, "no_repository")
			return nil, status.Errorf(codes.InvalidArgument, "missing repository in JWT token")
		}
		repo := r.(string)

		team := get("team", md)
		if team == "" {
			metrics.InterceptorRequest(requestTypeJWT, "no_team")
			return nil, status.Errorf(codes.InvalidArgument, "missing team in metadata")
		}

		if !s.TeamsClient.IsAuthorized(ctx, repo, team) {
			metrics.InterceptorRequest(requestTypeJWT, "repo_not_authorized")
			return nil, status.Errorf(codes.PermissionDenied, fmt.Sprintf("repo %q not authorized by team %q", repo, team))
		}

		metrics.InterceptorRequest(requestTypeJWT, "")
	} else {
		auth, err := extractAuthFromContext(ctx)
		if err != nil {
			metrics.InterceptorRequest(requestTypeApiKey, "invalid_auth_metadata")
			return nil, err
		}

		requestTime, _ := time.Parse(time.RFC3339Nano, auth.timestamp)
		if !withinTimeRange(requestTime) {
			metrics.InterceptorRequest(requestTypeApiKey, "signature_expired")
			return nil, status.Errorf(codes.DeadlineExceeded, "signature expired")
		}

		err = s.authenticate(ctx, *auth)
		if err != nil {
			return nil, err
		}

		metrics.InterceptorRequest(requestTypeApiKey, "")
	}

	return handler(ctx, req)
}

func get(key string, md metadata.MD) string {
	_, ok := md[key]
	if ok && len(md[key]) == 1 {
		return md[key][0]
	}
	return ""
}

func (s *ServerInterceptor) authenticate(ctx context.Context, auth authData) error {
	apiKeys, err := s.APIKeyStore.ApiKeys(ctx, auth.team)
	if err != nil {
		log.Errorf("Fetch API keys for team %s: %s", auth.team, err)
		if database.IsErrNotFound(err) {
			metrics.InterceptorRequest(requestTypeApiKey, "team_not_found")
			return status.Errorf(codes.Unauthenticated, "failed authentication")
		}
		metrics.InterceptorRequest(requestTypeApiKey, "database_error")
		return status.Errorf(codes.Unavailable, "something wrong happened when communicating with api key service")
	}

	err = api_v1.ValidateAnyMAC([]byte(auth.timestamp), auth.hmac, apiKeys.Valid().Keys())
	if err != nil {
		log.Errorf("Validate HMAC signature of team %s: %s", auth.team, err)
		metrics.InterceptorRequest(requestTypeApiKey, "invalid_api_key")
		return status.Errorf(codes.PermissionDenied, "failed authentication")
	}

	return nil
}

func withinTimeRange(t time.Time) bool {
	return math.Abs(time.Since(t).Seconds()) < api_v1.MaxTimeSkew
}

func (s *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return s.UnaryServerInterceptor
}

func (s *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	md, ok := metadata.FromIncomingContext(ss.Context())
	if !ok {
		return status.Errorf(codes.InvalidArgument, "invalid metadata in request")
	}

	jwt := get("jwt", md)

	if jwt != "" {
		t, err := s.TokenValidator.Validate(ss.Context(), jwt)
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid JWT token")
		}

		r, ok := t.Get("repository")
		if !ok {
			return status.Errorf(codes.InvalidArgument, "missing repository in JWT token")
		}
		repo := r.(string)

		team := get("team", md)
		if team == "" {
			return status.Errorf(codes.InvalidArgument, "missing team in metadata")
		}

		if !s.TeamsClient.IsAuthorized(ss.Context(), repo, team) {
			return status.Errorf(codes.PermissionDenied, fmt.Sprintf("repo %q not authorized by team %q", repo, team))
		}
	} else {
		auth, err := extractAuthFromContext(ss.Context())
		if err != nil {
			return err
		}

		requestTime, _ := time.Parse(time.RFC3339Nano, auth.timestamp)
		if !withinTimeRange(requestTime) {
			return status.Errorf(codes.DeadlineExceeded, "signature is too old")
		}

		err = s.authenticate(ss.Context(), *auth)
		if err != nil {
			return err
		}
	}

	return handler(srv, ss)
}

func (s *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return s.StreamServerInterceptor
}

func extractAuthFromContext(ctx context.Context) (*authData, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "request is not signed with API key; metadata is missing from request headers")
	}

	hmac := md["authorization"]
	if len(hmac) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "request is not signed with API key")
	}

	timestamp := md["timestamp"]
	if len(timestamp) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "API key signature timestamp is not provided")
	}

	team := md["team"]
	if len(team) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "team is not provided in API key signature metadata")
	}

	mac, err := hex.DecodeString(hmac[0])
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "wrong API key signature format")
	}

	return &authData{
		hmac:      mac,
		timestamp: timestamp[0],
		team:      team[0],
	}, nil
}
