package utils

import (
	"github.com/google/uuid"
	"github.com/o1egl/paseto"
	"os"
	"time"
)

// GenerateToken generates a token using paseto
func GenerateToken(audience string, subject string, payload map[string]string) (*string, error) {
	now := time.Now()
	exp := now.Add(time.Hour * 24 * 1) // 1 day
	nbf := now

	// generate a random token jti
	jti, _ := uuid.NewRandom()

	// create a new token
	jsonToken := paseto.JSONToken{
		Audience:   audience,
		Issuer:     "qreeket@qcodelabsllc.io",
		Jti:        jti.String(), // unique identifier for the token
		Subject:    subject,
		IssuedAt:   now,
		Expiration: exp,
		NotBefore:  nbf,
	}

	// iterate through the payload and add it to the token
	for key, value := range payload {
		jsonToken.Set(key, value)
	}

	// Encrypt data
	symmetricKey := []byte(os.Getenv("PASETO_KEY"))
	footer := []byte(os.Getenv("PASETO_FOOTER"))
	token, err := paseto.NewV2().Encrypt(symmetricKey, jsonToken, footer)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// DecryptToken decrypts a token using paseto
func DecryptToken(token string, key string) (*string, error) {
	symmetricKey := []byte(os.Getenv("PASETO_KEY"))
	//footer := []byte(os.Getenv("PASETO_FOOTER"))

	var newJsonToken paseto.JSONToken
	var newFooter string
	if err := paseto.NewV2().Decrypt(token, symmetricKey, &newJsonToken, &newFooter); err != nil {
		return nil, err
	}

	value := newJsonToken.Get(key)
	return &value, nil
}

// ValidateToken validates a token using paseto & checks if it's expired
func ValidateToken(token string) (bool, error) {
	symmetricKey := []byte(os.Getenv("PASETO_KEY"))
	//footer := []byte(os.Getenv("PASETO_FOOTER"))

	var newJsonToken paseto.JSONToken
	var newFooter string
	if err := paseto.NewV2().Decrypt(token, symmetricKey, &newJsonToken, &newFooter); err != nil {
		return false, err
	}

	// check if the token is expired
	if newJsonToken.Expiration.Before(time.Now()) {
		return false, nil
	}

	return true, nil
}
