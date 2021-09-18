package github

import (
	"io/ioutil"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
)

func CreateJWT(privateKeyPath string, appID string) (string, error) {
	signBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return "", err
	}

	signKey, err := jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		return "", err
	}

	t := jwt.New(jwt.GetSigningMethod("RS256"))
	t.Claims = &jwt.StandardClaims{
		IssuedAt:  time.Now().Add(-time.Second * 60).Unix(),
		ExpiresAt: time.Now().Add(time.Minute * 10).Unix(),
		Issuer:    appID,
	}

	return t.SignedString(signKey)
}
