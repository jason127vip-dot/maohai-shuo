package middleware

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSignedSessionRejectsTamperingAndExpiry(t *testing.T) {
	key := "test-secret-at-least-32-characters-long"
	value := strings.Repeat("a", 32) + "." + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	token := value + "." + sign(value, key)
	if _, ok := verify(token, key); !ok {
		t.Fatal("valid token rejected")
	}
	if _, ok := verify("b"+token[1:], key); ok {
		t.Fatal("tampered token accepted")
	}
	expired := strings.Repeat("a", 32) + ".1"
	if _, ok := verify(expired+"."+sign(expired, key), key); ok {
		t.Fatal("expired token accepted")
	}
}
