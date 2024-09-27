package open_auth

import (
	"github.com/golang-jwt/jwt/v5"
)

type OpenAuthenticator struct {
}

func NewOpenAuthenticator() *OpenAuthenticator {
	return &OpenAuthenticator{}
}

func (o OpenAuthenticator) ValidateJwt(authToken string) (jwt.MapClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(authToken, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	return token.Claims.(jwt.MapClaims), nil
}
