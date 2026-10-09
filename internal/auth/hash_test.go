package auth

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestHashFuncs(t *testing.T) {
	password := rand.Text()
	fmt.Printf("Random password: %v\n", password)
	hashed, err := HashPassword(password)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Hashed Password: %v\n", hashed)
	match, err := CheckPasswordHash(password, hashed)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Password Match: %v\n", match)
}
