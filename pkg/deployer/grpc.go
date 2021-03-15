package deployer

import (
	"crypto/tls"
	"encoding/hex"

	apikey_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/apikey"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func NewGrpcConnection(cfg Config) (*grpc.ClientConn, error) {
	dialOptions := make([]grpc.DialOption, 0)

	if !cfg.GrpcUseTLS {
		dialOptions = append(dialOptions, grpc.WithInsecure())
	} else {
		tlsOpts := &tls.Config{}
		cred := credentials.NewTLS(tlsOpts)
		dialOptions = append(dialOptions, grpc.WithTransportCredentials(cred))
	}

	if cfg.GrpcAuthentication {
		decoded, err := hex.DecodeString(cfg.APIKey)
		if err != nil {
			return nil, Errorf(ExitInvocationFailure, "%s: %s", MalformedAPIKeyMsg, err)
		}
		intercept := &apikey_interceptor.ClientInterceptor{
			APIKey:     decoded,
			RequireTLS: cfg.GrpcUseTLS,
			Team:       cfg.Team,
		}
		dialOptions = append(dialOptions, grpc.WithPerRPCCredentials(intercept))
	}

	grpcConnection, err := grpc.Dial(cfg.DeployServerURL, dialOptions...)
	if err != nil {
		return nil, Errorf(ExitInvocationFailure, "connect to NAIS deploy: %s", err)
	}

	return grpcConnection, nil
}
