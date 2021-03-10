package switch_interceptor

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Interceptor interface {
	Unary() grpc.UnaryServerInterceptor
	Stream() grpc.StreamServerInterceptor
}

type ServerInterceptor struct {
	urimap map[string]Interceptor
}

func isService(url, service string) bool {
	parts := strings.Split(strings.TrimLeft(url, "/"), "/")
	return len(parts) > 0 && service == parts[0]
}

func NewServerInterceptor() *ServerInterceptor {
	return &ServerInterceptor{
		urimap: make(map[string]Interceptor),
	}
}

func (t *ServerInterceptor) Add(prefix string, interceptor Interceptor) {
	t.urimap[prefix] = interceptor
}

func (t *ServerInterceptor) UnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	for prefix, interceptor := range t.urimap {
		if isService(info.FullMethod, prefix) {
			return interceptor.Unary()(ctx, req, info, handler)
		}
	}
	return nil, status.Errorf(codes.Unimplemented, "BUG: no interceptor added for service endpoint %s", info.FullMethod)
}

func (t *ServerInterceptor) Unary() grpc.UnaryServerInterceptor {
	return t.UnaryServerInterceptor
}

func (t *ServerInterceptor) StreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	for prefix, interceptor := range t.urimap {
		if isService(info.FullMethod, prefix) {
			return interceptor.Stream()(srv, ss, info, handler)
		}
	}
	return status.Errorf(codes.Unimplemented, "BUG: no interceptor added for service endpoint %s", info.FullMethod)
}

func (t *ServerInterceptor) Stream() grpc.StreamServerInterceptor {
	return t.StreamServerInterceptor
}
