package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"
	expiresIn := time.Hour

	tokenString, err := MakeJWT(userID, secret, expiresIn)
	if err != nil {
		t.Fatalf("MakeJWT() returned an unexpected error: %v", err)
	}

	if tokenString == "" {
		t.Fatal("MakeJWT() returned an empty token")
	}

	// Parse the token to verify its claims and signature.
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse generated token: %v", err)
	}

	if !token.Valid {
		t.Fatal("Expected generated token to be valid")
	}

	if claims.Issuer != "chirpy-access" {
		t.Errorf("Expected issuer %q, got %q", "chirpy-access", claims.Issuer)
	}

	if claims.Subject != userID.String() {
		t.Errorf("Expected subject %q, got %q", userID.String(), claims.Subject)
	}

	if claims.IssuedAt == nil {
		t.Fatal("Expected IssuedAt claim to be set")
	}

	if claims.ExpiresAt == nil {
		t.Fatal("Expected ExpiresAt claim to be set")
	}

	expectedExpiry := time.Now().Add(expiresIn)
	if diff := claims.ExpiresAt.Time.Sub(expectedExpiry); diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("Expected expiration around %v, got %v", expectedExpiry, claims.ExpiresAt.Time)
	}
}

func TestValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"

	tokenString, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT() returned an unexpected error: %v", err)
	}

	gotID, err := ValidateJWT(tokenString, secret)
	if err != nil {
		t.Fatalf("ValidateJWT() returned an unexpected error: %v", err)
	}

	if gotID != userID {
		t.Errorf("Expected user ID %v, got %v", userID, gotID)
	}
}

func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()

	tokenString, err := MakeJWT(userID, "correct-secret", time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT() returned an unexpected error: %v", err)
	}

	_, err = ValidateJWT(tokenString, "wrong-secret")
	if err == nil {
		t.Fatal("Expected an error when validating with the wrong secret")
	}
}

func TestValidateJWTExpired(t *testing.T) {
	userID := uuid.New()

	tokenString, err := MakeJWT(userID, "test-secret", -time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT() returned an unexpected error: %v", err)
	}

	_, err = ValidateJWT(tokenString, "test-secret")
	if err == nil {
		t.Fatal("Expected an error when validating an expired token")
	}
}

func TestValidateJWTInvalidToken(t *testing.T) {
	_, err := ValidateJWT("this-is-not-a-jwt", "test-secret")
	if err == nil {
		t.Fatal("Expected an error when validating a malformed token")
	}
}

func TestValidateJWTInvalidSubject(t *testing.T) {
	// Create a correctly signed token with a subject that isn't a UUID.
	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		Subject:   "not-a-uuid",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("Failed to sign test token: %v", err)
	}

	_, err = ValidateJWT(tokenString, "test-secret")
	if err == nil {
		t.Fatal("Expected an error when subject is not a valid UUID")
	}
}
