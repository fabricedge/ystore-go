package crypto

import (
	"bytes"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	password := "hunter2"
	plaintext := []byte("Hello, world! This is test data for the crypto package.")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("round trip mismatch: got %x, want %x", decrypted, plaintext)
	}
}

func TestWrongPassword(t *testing.T) {
	encrypted, err := Encrypt([]byte("test data"), "correct password")
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(encrypted, "wrong password")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestEmptyPassword(t *testing.T) {
	_, err := Encrypt([]byte("test"), "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestDifferentSaltNonce(t *testing.T) {
	pwd := "same password"
	data := []byte("same data")

	e1, _ := Encrypt(data, pwd)
	e2, _ := Encrypt(data, pwd)

	if bytes.Equal(e1, e2) {
		t.Fatal("two encryptions of same data produced identical output")
	}
}
