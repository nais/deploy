package apikey_interceptor

import (
	"context"

	"github.com/navikt/deployment/pkg/hookd/database"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type ServerInterceptor struct {
	APIKeyStore database.ApiKeyStore
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	log.Errorf(info.FullMethod)
	// t.APIKeyStore.ApiKeys()
	return handler(ctx, req)
}

func (t *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Errorf(info.FullMethod)
	// t.APIKeyStore.ApiKeys()
	return handler(srv, ss)
}

func (t *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}
