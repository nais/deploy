package interceptor

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ClientInterceptor struct {
	Config clientcredentials.Config
	token  oauth2.Token
}

func (t *ClientInterceptor) TokenLoop() {
	getToken := func() (*oauth2.Token, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		return t.Config.Token(ctx)
	}

	timer := time.NewTimer(0)

	for range timer.C {
		tok, err := getToken()
		if err != nil {
			log.Errorf("Error while refreshing oauth2 token: %s", err)
			timer.Reset(1 * time.Minute)
			continue
		}
		const RefreshIntervalFactor = 0.8
		t.token = *tok
		lifetime := tok.Expiry.Sub(time.Now())
		refreshInterval := float64(lifetime) * RefreshIntervalFactor
		duration := time.Duration(refreshInterval)
		timer.Reset(duration)
		log.Infof("Successfully refreshed oauth2 token, next refresh in %s", duration.String())
	}
}

func (t *ClientInterceptor) UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", t.token.AccessToken)
	return invoker(ctx, method, req, reply, cc, opts...)
}

func (t *ClientInterceptor) Unary() grpc.UnaryClientInterceptor {
	return t.UnaryClientInterceptor
}

func (t *ClientInterceptor) StreamClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	// fixme
	return nil, nil
}

func (t *ClientInterceptor) Stream() grpc.StreamClientInterceptor {
	return t.StreamClientInterceptor
}
