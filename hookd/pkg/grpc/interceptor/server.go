package interceptor

import (
	"context"

	"github.com/dgrijalva/jwt-go"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/azure/validate"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerInterceptor struct {
	Audience     string
	Certificates map[string]discovery.CertificateList
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}

	accessToken := values[0]
	claims, err := jwt.Parse(accessToken, validate.JWTValidator(t.Certificates, t.Audience))
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "access token is invalid: %v", err)
	}

	log.Infof("token: %#v", claims)

	return handler(ctx, req)

}

func (t *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// fixme
	return handler(srv, ss)
}

func (t *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}
