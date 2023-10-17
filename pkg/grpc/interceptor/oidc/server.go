package oidc

import (
	"context"
	"fmt"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/nais/deploy/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

const gitHubDiscoveryURL = "https://token.actions.githubusercontent.com/.well-known/openid-configuration"
const gitHubIssuer = "https://token.actions.githubusercontent.com/"

type ServerInterceptor struct {
	jwkCache *jwk.Cache
}

func (t *ServerInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": ""}, nil
}

func (t *ServerInterceptor) RequireTransportSecurity() bool {
	return true
}

func NewServerInterceptor() (*ServerInterceptor, error) {
	o := &ServerInterceptor{}
	err := o.setupJwkAutoRefresh()
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	token, err := t.verifyJWT(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "verify jwt: %v", err)
	}

	request, ok := req.(*pb.DeploymentRequest)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "requests to this endpoint must be 'DeploymentRequest'")
	}

	err = t.verifyPrecoditions(request, token)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "verify info in request: %v", err)
	}
	return handler(ctx, req)
}

func (t *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return handler(srv, ss)
}

func (t *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}

func (t *ServerInterceptor) verifyJWT(ctx context.Context) (jwt.Token, error) {
	pubKeys, err := t.jwkCache.Get(context.Background(), gitHubDiscoveryURL)
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	keySetOpts := jwt.WithKeySet(pubKeys, jws.WithInferAlgorithmFromKey(true))
	otherParseOpts := t.jwtOptions()
	strTok, err := extractJWT(ctx)
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse([]byte(strTok), append(otherParseOpts, keySetOpts)...)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (t *ServerInterceptor) verifyPrecoditions(req *pb.DeploymentRequest, token jwt.Token) error {
	teamFromRequest := req.GetTeam()
	repoFromToken, exists := token.Get("repository")
	if !exists {
		return fmt.Errorf("token doesn't contain the required claim '%v'", "repository")
	}
	repoName := fmt.Sprintf("%v", repoFromToken)
	teamFromTeams, err := teamOwning(repoName)
	if err != nil {
		return err
	}
	if teamFromTeams != teamFromRequest {
		return fmt.Errorf("'%s' is owned by '%s', not '%s' as it should", repoName, teamFromTeams, teamFromRequest)
	}
	return nil
}

func (t *ServerInterceptor) setupJwkAutoRefresh() error {
	ctx := context.Background()
	cache := jwk.NewCache(ctx)
	err := cache.Register(gitHubDiscoveryURL, jwk.WithRefreshInterval(time.Hour))
	if err != nil {
		return fmt.Errorf("jwks caching: %w", err)
	}
	// trigger initial refresh
	_, err = cache.Refresh(ctx, gitHubDiscoveryURL)
	if err != nil {
		return fmt.Errorf("jwks caching: %w", err)
	}
	t.jwkCache = cache
	return nil
}

func extractJWT(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "MetadataRetriever is missing from request headers")
	}
	headerValue := md["authorization"]
	if len(headerValue) == 0 {
		return "", status.Errorf(codes.Unauthenticated, "OIDC token is not provided")
	}
	authorizationValue := strings.Split(headerValue[0], " ")
	return authorizationValue[1], nil
}

func (t *ServerInterceptor) jwtOptions() []jwt.ParseOption {
	return []jwt.ParseOption{
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(5 * time.Second),
		jwt.WithIssuer(gitHubIssuer),
		jwt.WithAudience("nais-deploy"), // todo
		jwt.WithRequiredClaim("email"),  // todo which claims?
	}
}

func teamOwning(repo string) (string, error) {
	return "", nil // todo
}
