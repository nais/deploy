package token_interceptor

import (
	"context"

	"github.com/dgrijalva/jwt-go"
	"github.com/navikt/deployment/pkg/azure/discovery"
	"github.com/navikt/deployment/pkg/azure/oauth2"
	"github.com/navikt/deployment/pkg/azure/validate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerInterceptor struct {
	Audience     string
	Certificates map[string]discovery.CertificateList
	PreAuthApps  []oauth2.PreAuthorizedApplication
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	err = t.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (t *ServerInterceptor) authenticate(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}

	accessToken := values[0]
	var claims jwt.MapClaims
	_, err := jwt.ParseWithClaims(accessToken, &claims, validate.JWTValidator(t.Certificates, t.Audience))
	if err != nil {
		return status.Errorf(codes.Unauthenticated, "access token is invalid: %v", err)
	}

	for _, authApp := range t.PreAuthApps {
		if authApp.ClientId == claims["azp"] {
			return nil
		}
	}

	return status.Errorf(codes.PermissionDenied, "application is not authorized")
}

func (t *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := t.authenticate(ss.Context())
	if err != nil {
		return err
	}

	return handler(srv, ss)
}

func (t *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}
