package auth_interceptor

import (
	"context"
	"encoding/hex"
	"math"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerInterceptor struct {
	APIKeyStore    database.ApiKeyStore
	TokenValidator TokenValidator
	TeamsClient    TeamsClient
}

type TokenValidator interface {
	Validate(token string) (jwt.Token, error)
}

type TeamsClient interface {
	IsAuthorized(repo, team string) bool
}

type authData struct {
	hmac      []byte
	timestamp string
	team      string
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

func (s *ServerInterceptor) authenticate(ctx context.Context, auth authData) error {
	apiKeys, err := s.APIKeyStore.ApiKeys(ctx, auth.team)
	if err != nil {
		log.Errorf("Fetch API keys for team %s: %s", auth.team, err)
		if database.IsErrNotFound(err) {
			return status.Errorf(codes.Unauthenticated, "failed authentication")
		}
		return status.Errorf(codes.Unavailable, "something wrong happened when communicating with api key service")
	}

	err = api_v1.ValidateAnyMAC([]byte(auth.timestamp), auth.hmac, apiKeys.Valid().Keys())
	if err != nil {
		log.Errorf("Validate HMAC signature of team %s: %s", auth.team, err)
		return status.Errorf(codes.PermissionDenied, "failed authentication")
	}

	return nil
}

func withinTimeRange(t time.Time) bool {
	return math.Abs(time.Since(t).Seconds()) < api_v1.MaxTimeSkew
}

func (s *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	request, ok := req.(*pb.DeploymentRequest)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "requests to this endpoint must be DeploymentRequest")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "invalid metadata in request")
	}

	jwt := get("jwt", md)

	if jwt != "" {
		t, err := s.TokenValidator.Validate(jwt)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid JWT token")
		}

		r, ok := t.Get("repository")
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "missing repository in JWT token")
		}
		repo := r.(string)

		team := get("team", md)
		if team == "" {
			return nil, status.Errorf(codes.Unauthenticated, "missing team in metadata")
		}

		if s.TeamsClient.IsAuthorized(repo, team) {
			return handler(ctx, req)
		} else {
			return nil, status.Errorf(codes.PermissionDenied, "repo not authorized by team")
		}
	}

	auth, err := extractAuthFromContext(ctx)
	if err != nil {
		return nil, err
	}

	auth.team = request.GetTeam()

	requestTime, _ := time.Parse(time.RFC3339Nano, auth.timestamp)
	if !withinTimeRange(requestTime) {
		return nil, status.Errorf(codes.DeadlineExceeded, "signature is too old")
	}

	err = s.authenticate(ctx, *auth)
	if err != nil {
		return nil, err
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

func (s *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return s.UnaryServerInterceptor
}

func (s *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	auth, err := extractAuthFromContext(ss.Context())
	if err != nil {
		return err
	}

	err = s.authenticate(ss.Context(), *auth)
	if err != nil {
		return err
	}
	return handler(srv, ss)
}

func (s *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return s.StreamServerInterceptor
}
