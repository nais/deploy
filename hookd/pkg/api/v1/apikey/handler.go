package main

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
	"github.com/navikt/deployment/hookd/pkg/azure/conf"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/azure/validate"
	log "github.com/sirupsen/logrus"
)

var (
	tokenAuth         *jwtauth.JWTAuth
	azureCertificates map[string]discovery.CertificateList
	azureConf         *conf.Azure
)

func router(certificates map[string]discovery.CertificateList, azureConf conf.Azure) http.Handler {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("welcome anonymous"))
			token := jwtauth.TokenFromHeader(r)

			_, err := jwt.Parse(token, validate.JWTValidator(certificates))
			fmt.Printf("Groups: %#v", validate.Groups)

			if err != nil {
				fmt.Printf("error: %s", err.Error())
				fmt.Fprintf(w, "Unauthorized access: %s", err.Error())
			} else {
				fmt.Printf("groups: %s", validate.Groups)
				w.Write([]byte("welcome anonymous"))
			}
		})
	})

	return r
}

func main() {
	azureConf := conf.Azure{
		ClientID:     "ecd35adf-754e-4c75-8098-8e6e1d33cdf9",
		ClientSecret: "x.u5vW3@o]8u5f]jihNsvvgomyCCtw03",
		Tenant:       "62366534-1ec3-4962-8869-9b5535279d0b",
		RedirectURL:  "nothing",
		DiscoveryURL: "https://login.microsoftonline.com/62366534-1ec3-4962-8869-9b5535279d0b/discovery/v2.0/keys",
	}
	certificates, err := discovery.FetchCertificates(azureConf)
	if err != nil {
		log.Errorf(err.Error())
	}
	addr := ":8081"
	fmt.Printf("Starting server on %v\n", addr)
	http.ListenAndServe(addr, router(certificates, azureConf))
}
