package interceptor

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type ClientInterceptor struct {
	Config     clientcredentials.Config
	RequireTLS bool
	token      oauth2.Token
}

func (t *ClientInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": t.token.AccessToken}, nil
}

func (t *ClientInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
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
