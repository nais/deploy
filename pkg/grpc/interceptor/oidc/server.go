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

type OidcServerInterceptor struct {
	jwkCache *jwk.Cache
}

func NewOidcServerInterceptor() (*OidcServerInterceptor, error) {
	o := &OidcServerInterceptor{}
	err := o.setupJwkAutoRefresh()
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (t *OidcServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	err = t.authenticate(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "error while doing auth: %v", err)
	}

	request, ok := req.(*pb.DeploymentRequest)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "requests to this endpoint must be DeploymentRequest")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "request is not signed with API key; MetadataRetriever is missing from request headers")
	}
	md.Get("authorization")
	request.GetTeam() // todo validate info in request
	return handler(ctx, req)
}

func (t *OidcServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	// todo
	return t.UnaryServerInterceptor
}

func (t *OidcServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// todo
	return handler(srv, ss)
}

func (t *OidcServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}

func (t *OidcServerInterceptor) authenticate(ctx context.Context) error {
	pubKeys, err := t.jwkCache.Get(context.Background(), gitHubDiscoveryURL)
	if err != nil {
		return fmt.Errorf("parse token: %w", err)
	}
	keySetOpts := jwt.WithKeySet(pubKeys, jws.WithInferAlgorithmFromKey(true))
	otherParseOpts := t.jwtOptions()
	strTok, err := extractJWT(ctx)
	if err != nil {
		return err
	}
	_, err = jwt.Parse([]byte(strTok), append(otherParseOpts, keySetOpts)...)
	if err != nil {
		return err
	}
	return nil
}

func (t *OidcServerInterceptor) setupJwkAutoRefresh() error {
	ctx := context.Background()
	cache := jwk.NewCache(ctx)
	err := cache.Register(gitHubDiscoveryURL, jwk.WithRefreshInterval(time.Hour))
	if err != nil {
		return fmt.Errorf("jwks caching: %w", err)
	}
	// force initial refresh
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

func (t *OidcServerInterceptor) jwtOptions() []jwt.ParseOption {
	return []jwt.ParseOption{
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(5 * time.Second),
		jwt.WithIssuer(gitHubIssuer),
		jwt.WithAudience(""),           // todo
		jwt.WithRequiredClaim("email"), // todo which claims?
	}
}
