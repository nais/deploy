package apikey_interceptor

import (
	"context"
	"encoding/hex"

	api_v1 "github.com/navikt/deployment/pkg/hookd/api/v1"
	"github.com/navikt/deployment/pkg/hookd/database"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerInterceptor struct {
	APIKeyStore database.ApiKeyStore
}

func (t *ServerInterceptor) authenticate(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	signature := md["authorization"]
	if len(signature) == 0 {
		return status.Errorf(codes.Unauthenticated, "authorization is not provided")
	}

	timestamp := md["timestamp"]
	if len(timestamp) == 0 {
		return status.Errorf(codes.Unauthenticated, "timestamp is not provided")
	}

	team := md["team"]
	if len(team) == 0 {
		return status.Errorf(codes.Unauthenticated, "team is not provided")
	}

	mac, err := hex.DecodeString(signature[0])
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "wrong signature format")
	}

	apiKeys, err := t.APIKeyStore.ApiKeys(ctx, team[0])
	if err != nil {
		if database.IsErrNotFound(err) {
			return status.Errorf(codes.Unauthenticated, "failed authentication")
		}
		return status.Errorf(codes.Unavailable, "something wrong happened when communicating with api key service")
	}

	err = api_v1.ValidateAnyMAC([]byte(timestamp[0]), mac, apiKeys.Valid().Keys())
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "failed authentication")
	}

	return nil
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	err = t.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	return handler(ctx, req)
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
