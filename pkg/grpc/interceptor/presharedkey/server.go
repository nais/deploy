package presharedkey_interceptor

import (
	"context"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ServerInterceptor struct {
	Keys []string
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
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
		return status.Errorf(codes.Unauthenticated, "authorization key is not provided")
	}

	accessKey := values[0]
	if slices.Contains(t.Keys, accessKey) {
		return nil
	}

	return status.Errorf(codes.PermissionDenied, "application is not authorized")
}

func (t *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *ServerInterceptor) StreamServerInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := t.authenticate(ss.Context())
	if err != nil {
		return err
	}

	return handler(srv, ss)
}

func (t *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}
