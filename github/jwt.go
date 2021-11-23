package github

import (
	"io/ioutil"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"

	"github.com/plumber-cd/github-apps-trampoline/logger"
)

func CreateJWT(privateKeyPath string, appID int) (string, error) {
	logger.Get().Printf("Creating JWT using privateKeyPath=%s appID=%d", privateKeyPath, appID)

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
		IssuedAt:  time.Now().Add(-time.Second * 60).Unix(), // Allow 1 minute drift
		ExpiresAt: time.Now().Add(time.Minute * 9).Unix(), // Max is 10 mins, allow 1 minute drift
		Issuer:    strconv.Itoa(appID),
	}

	token, err := t.SignedString(signKey)
	if err != nil {
		return "", err
	}

	logger.Get().Printf("Created JWT: %s", token)
	return token, nil
}
