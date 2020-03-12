package main

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/jwtauth"
	"github.com/navikt/deployment/hookd/pkg/azure/conf"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	log "github.com/sirupsen/logrus"
)

var (
	tokenAuth         *jwtauth.JWTAuth
	azureCertificates map[string]discovery.CertificateList
	azureConf         *conf.Azure
)

func JWTValidator(certificates map[string]discovery.CertificateList) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		var certificateList discovery.CertificateList
		var kid string
		var ok bool

		if claims, ok := token.Claims.(jwt.MapClaims); !ok {
			return nil, fmt.Errorf("Unable to retrieve claims from token")
		} else {
			if valid := claims.VerifyAudience("f29d724c-fdbf-4e43-b65a-3f123442bd88", true); !valid {
				return nil, fmt.Errorf("The token is not valid for this application")
			}
		}

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

func router(certificates map[string]discovery.CertificateList, azureConf conf.Azure) http.Handler {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			token := jwtauth.TokenFromHeader(r)

			_, err := jwt.Parse(token, JWTValidator(certificates))
			if err != nil {
				fmt.Printf("error: %s", err.Error())
				fmt.Fprintf(w, "Unauthorized access: %s", err.Error())
			} else {
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
