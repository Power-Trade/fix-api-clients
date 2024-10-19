package pt

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func ReadKeys(keyFilename string) (apiKey string, privateKey []byte, err error) {
	path := "./keys"
	apiKeyB, err := os.ReadFile(fmt.Sprintf("%s/%v.api", path, keyFilename))
	if err != nil {
		panic("Can't read api key")
	}
	apiKey = strings.Split(string(apiKeyB), "\n")[0]
	privateKey, err = os.ReadFile(fmt.Sprintf("%s/%v.pem", path, keyFilename))
	if err != nil {
		panic("Can't read private key")
	}
	return
}

func GeneratePassword(
	apiKey string,
	privateKey []byte,
	serverUri string,
	nameOfAlg string,
	clientType string, // "api" / "app"
	dur time.Duration,
) (password string, err error) {

	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		ExpiresAt: &jwt.NumericDate{Time: now.Add(dur)},
		IssuedAt:  &jwt.NumericDate{Time: now},
		Issuer:    serverUri,
		Subject:   string(apiKey),
	}

	var privKey interface{}
	var token *jwt.Token

	switch nameOfAlg {
	case "RS256":
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		privKey, err = jwt.ParseRSAPrivateKeyFromPEM(privateKey)
	case "ES256", "":
		token = jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		privKey, err = jwt.ParseECPrivateKeyFromPEM(privateKey)
	default:
		panic("unknown alg " + nameOfAlg)
	}
	if err != nil {
		panic(err)
	}

	tokenString, err := token.SignedString(privKey)
	if err != nil {
		panic(err)
	}

	password = tokenString
	return
}
