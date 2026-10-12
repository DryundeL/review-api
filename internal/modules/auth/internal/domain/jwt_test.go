package domain

import (
	"strings"
	"testing"
	"time"
)

func TestJWTRoundTrip(t *testing.T) {
	t.Parallel()
	secret := []byte(strings.Repeat("s", 32))
	token, err := SignJWT(secret, 42, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseJWT(secret, token, time.Now())
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if _, err := ParseJWT([]byte(strings.Repeat("o", 32)), token, time.Now()); err == nil {
		t.Fatal("accepted foreign secret")
	}
	expired, err := SignJWT(secret, 42, time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseJWT(secret, expired, time.Now()); err == nil {
		t.Fatal("accepted expired token")
	}
	parts := strings.Split(token, ".")
	payload := []byte(parts[1])
	if payload[0] == 'A' {
		payload[0] = 'B'
	} else {
		payload[0] = 'A'
	}
	parts[1] = string(payload)
	if _, err := ParseJWT(secret, strings.Join(parts, "."), time.Now()); err == nil {
		t.Fatal("accepted tampered token")
	}
}
