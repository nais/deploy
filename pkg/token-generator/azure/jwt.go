package azure

import (
	"fmt"

	"github.com/dgrijalva/jwt-go"
)

// Cryptographic signature check of a JSON Web Token.
// The signature is checked against a certificate list.
//
// Implemented according to:
// https://github.com/navikt/navs-aad-authorization-flow/blob/master/NAVS-AAD-Example-APP/verifyToken.md
func JWTValidator(certificates map[string]CertificateList) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {

		var certificateList CertificateList
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

func AuthorizeURL(tenant, endpoint string) string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/%s", tenant, endpoint)
}
