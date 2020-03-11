package main

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	log "github.com/sirupsen/logrus"
)

var (
	tokenAuth         *jwtauth.JWTAuth
	azureCertificates map[string]discovery.CertificateList
)

func JWTValidator(certificates map[string]discovery.CertificateList) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {

		var certificateList discovery.CertificateList
		var kid string
		var ok bool

		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		if kid, ok = token.Header["kid"].(string); !ok {
			return nil, fmt.Errorf("field 'kid' is of invalid type %T, should be string", token.Header["kid"])
		}

		if certificateList, ok = certificates[kid]; !ok {
			return nil, fmt.Errorf("kid '%s' not found in certificate list", kid)
		}

		for _, certificate := range certificateList {
			return certificate.PublicKey, nil
		}

		return nil, fmt.Errorf("no certificate candidates for kid '%s'", kid)
	}
}

func router(certificates map[string]discovery.CertificateList) http.Handler {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			token := jwtauth.TokenFromHeader(r)
			_, err := jwt.Parse(token, JWTValidator(certificates))
			fmt.Printf("error is: %e", err)

			w.Write([]byte("welcome anonymous"))
		})
	})

	return r
}

func main() {
	certificates, err := discovery.FetchCertificates()
	if err != nil {
		log.Errorf(err.Error())
	}
	addr := ":8081"
	fmt.Printf("Starting server on %v\n", addr)
	http.ListenAndServe(addr, router(certificates))
}
