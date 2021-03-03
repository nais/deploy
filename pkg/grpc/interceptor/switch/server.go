package switch_interceptor

import (
	"context"
	api_v1 "github.com/navikt/deployment/pkg/hookd/api/v1"
	"github.com/navikt/deployment/pkg/hookd/database"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Interceptor interface {
	Unary() grpc.UnaryServerInterceptor
	Stream() grpc.StreamServerInterceptor
}

type SwitchInterceptor struct {
	urimap map[string]Interceptor
}

func (t *SwitchInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	log.Errorf(info.FullMethod)
	// t.APIKeyStore.ApiKeys()
	return handler(ctx, req)
}

func (t *SwitchInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *SwitchInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Errorf(info.FullMethod)
	// /pb.dispatch/deployments
	uri := t.urimap[info.FullMethod]



	// t.APIKeyStore.ApiKeys()
	return handler(srv, ss)
}

func (t *SwitchInterceptor) Stream() grpc.StreamServerInterceptor {

	return t.StreamServerInterceptor
}
