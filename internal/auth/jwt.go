package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(tokenSecret))
	return signedToken, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(*jwt.Token) (interface{}, error) { return []byte(tokenSecret), nil })
	if err != nil {
		return uuid.UUID{}, err
	}

	idString, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, err
	}

	id, err := uuid.Parse(idString)
	if err != nil {
		return uuid.UUID{}, err
	}

	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authString := headers.Get("Authorization")
	if authString == "" {
		return "", errors.New("No Auth Header")
	}
	trimmedAuthString, _ := strings.CutPrefix(authString, "Bearer ")
	return trimmedAuthString, nil
}
