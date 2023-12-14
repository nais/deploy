package deployclient

import (
	"crypto/tls"
	"encoding/hex"

	auth_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func NewGrpcConnection(cfg Config) (*grpc.ClientConn, error) {
	dialOptions := make([]grpc.DialOption, 0)

	if !cfg.GrpcUseTLS {
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsOpts := &tls.Config{}
		cred := credentials.NewTLS(tlsOpts)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(cred))
	}

	if cfg.GrpcAuthentication {
		var interceptor auth_interceptor.ClientInterceptor
		if cfg.GithubToken != "" {
			interceptor = &auth_interceptor.JWTInterceptor{
				JWT:        cfg.GithubToken,
				RequireTLS: cfg.GrpcUseTLS,
				Team:       cfg.Team,
			}
		} else {
			decoded, err := hex.DecodeString(cfg.APIKey)
			if err != nil {
				return nil, Errorf(ExitInvocationFailure, "%s: %s", MalformedAPIKeyMsg, err)
			}
			interceptor = &auth_interceptor.APIKeyInterceptor{
				APIKey:     decoded,
				RequireTLS: cfg.GrpcUseTLS,
				Team:       cfg.Team,
			}
		}
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(interceptor))
	}

	grpcConnection, err := grpc.Dial(cfg.DeployServerURL, dialOptions...)
	if err != nil {
		return nil, Errorf(ExitInvocationFailure, "connect to NAIS deploy: %s", err)
	}

	return grpcConnection, nil
}
