package auth

import "crypto/rand"

func MakeRefreshToken() string {
	return rand.Text()
}
