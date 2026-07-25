package auth

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash = %q, want argon2id PHC prefix", hash)
	}
	ok, err := VerifyPassword(hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Errorf("verify correct password = %v, %v; want true, nil", ok, err)
	}
	ok, err = VerifyPassword(hash, "wrong password")
	if err != nil || ok {
		t.Errorf("verify wrong password = %v, %v; want false, nil", ok, err)
	}
}

func TestHashesAreSalted(t *testing.T) {
	a, err := HashPassword("same input")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword("same input")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical; salt is not working")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	good, err := HashPassword("x")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	parts := strings.Split(good, "$")
	bad := []string{
		"",
		"plaintext",
		"$bcrypt$v=19$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=18$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=0,t=0,p=0$" + parts[4] + "$" + parts[5],
		"$argon2id$v=19$m=19456,t=2,p=1$!notb64!$" + parts[5],
		"$argon2id$v=19$m=19456,t=2,p=1$" + parts[4] + "$!notb64!",
		"$argon2id$v=19$m=19456,t=2,p=1$" + parts[4],
	}
	for _, h := range bad {
		if _, err := VerifyPassword(h, "x"); !errors.Is(err, ErrMalformedHash) {
			t.Errorf("VerifyPassword(%q) err = %v, want ErrMalformedHash", h, err)
		}
	}
}

func TestVerifyHonoursStoredParameters(t *testing.T) {
	// A hash that was created with older and different parameters must still verify.
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte("old password"), salt, 1, 8192, 1, 32)
	hash := "$argon2id$v=19$m=8192,t=1,p=1$" +
		base64.RawStdEncoding.EncodeToString(salt) + "$" +
		base64.RawStdEncoding.EncodeToString(key)
	ok, err := VerifyPassword(hash, "old password")
	if err != nil || !ok {
		t.Errorf("verify with stored weaker params = %v, %v; want true, nil", ok, err)
	}
	ok, err = VerifyPassword(hash, "not it")
	if err != nil || ok {
		t.Errorf("wrong password with stored weaker params = %v, %v; want false, nil", ok, err)
	}
}
