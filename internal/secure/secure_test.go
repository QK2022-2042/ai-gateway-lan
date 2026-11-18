package secure

import "testing"

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct-horse-battery", hash) {
		t.Fatal("valid password rejected")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("wrong password accepted")
	}
	if VerifyPassword("correct-horse-battery", "garbage") {
		t.Fatal("malformed hash accepted")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	// Different salts every time.
	hash2, _ := HashPassword("correct-horse-battery")
	if hash == hash2 {
		t.Fatal("hashes are not salted")
	}
}

func TestVaultRoundTrip(t *testing.T) {
	v, err := NewVault([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := v.EncryptStrings([]string{"sk-a", "sk-b"})
	if err != nil {
		t.Fatal(err)
	}
	if enc == "" || enc[:3] != "v1:" {
		t.Fatalf("unexpected ciphertext: %q", enc)
	}
	got, err := v.DecryptStrings(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "sk-a" || got[1] != "sk-b" {
		t.Fatalf("round trip mismatch: %v", got)
	}
	// Tampering must fail authentication.
	tampered := enc[:len(enc)-2] + "AA"
	if _, err := v.DecryptStrings(tampered); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}

func TestAPIKeyFormat(t *testing.T) {
	plain, hash, prefix := NewAPIKey()
	if len(plain) != 43 || plain[:3] != "gw-" {
		t.Fatalf("unexpected key: %q", plain)
	}
	if HashToken(plain) != hash {
		t.Fatal("hash mismatch")
	}
	if len(prefix) < 6 {
		t.Fatalf("short prefix: %q", prefix)
	}
	if MaskSecret("sk-1234567890abcd") != "sk-1…abcd" {
		t.Fatalf("unexpected mask: %q", MaskSecret("sk-1234567890abcd"))
	}
}
