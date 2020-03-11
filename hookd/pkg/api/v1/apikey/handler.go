package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	log "github.com/sirupsen/logrus"
)

var (
	// tokenAuth
	tokenAuth         *jwtauth.JWTAuth
	azureCertificates map[string]discovery.CertificateList
)


func router() http.Handler {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			//			token := jwtauth.TokenFromHeader(r)
			//			jwt.Parse(token, JWTValidator(certificates))
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
	fmt.Printf("%#v", certificates)
	addr := ":8081"
	fmt.Printf("Starting server on %v\n", addr)
	http.ListenAndServe(addr, router())
}
