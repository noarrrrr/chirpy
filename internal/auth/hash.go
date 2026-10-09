package auth

import "github.com/alexedwards/argon2id"

func HashPassword(pass string) (string, error) {
	params := argon2id.DefaultParams
	result, err := argon2id.CreateHash(pass, params)
	if err != nil {
		return "", err
	}
	return result, nil
}

func CheckPasswordHash(pass, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(pass, hash)
	if err != nil {
		return false, err
	}
	return match, nil
}
