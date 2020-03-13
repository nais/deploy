package validate

import (
	"fmt"

	"github.com/dgrijalva/jwt-go"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
)

func JWTValidator(certificates map[string]discovery.CertificateList) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		var certificateList discovery.CertificateList
		var kid string
		var ok bool

		fmt.Printf("Token: %#v", token)
		if claims, ok := token.Claims.(*jwt.MapClaims); !ok {
			fmt.Printf("claims in validator: %#v", claims)
			return nil, fmt.Errorf("Unable to retrieve claims from token")
		} else {
			fmt.Printf("claims: %#v", claims)
			// Todo: use azure.ClientID instead of hard  coded value
			if valid := claims.VerifyAudience("f29d724c-fdbf-4e43-b65a-3f123442bd88", true); !valid {
				return nil, fmt.Errorf("The token is not valid for this application")
			}
			/*			// Converting groups from claims to group slice
						groupInterface := claims["groups"].([]interface{})
						Groups = make([]string, len(groupInterface))
						for i, v := range groupInterface {
							Groups[i] = v.(string)
						}
			*/
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
