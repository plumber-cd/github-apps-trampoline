package github

import (
	"os"
	"strconv"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"

	"github.com/plumber-cd/github-apps-trampoline/logger"
)

func CreateJWT(privateKeyPath string, appID int) (string, error) {
	logger.Get().Printf("Creating JWT using privateKeyPath=%s appID=%d", privateKeyPath, appID)

	signBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", err
	}

	signKey, err := jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		return "", err
	}

	t := jwt.New(jwt.GetSigningMethod("RS256"))
	t.Claims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Second * 60)), // Allow 1 minute drift
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 9)),   // Max is 10 mins, allow 1 minute drift
		Issuer:    strconv.Itoa(appID),
	}

	token, err := t.SignedString(signKey)
	if err != nil {
		return "", err
	}

	logger.Get().Printf("Created JWT: %s", token)
	return token, nil
}
