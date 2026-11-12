package crypto

import (
	"encoding/hex"
	"testing"
)

var (
	privateKey1 = "2700c94e67119e2c523caf8aed574e5bf3d633b261df4d451128f8f284007f4b"
	publicKey1  = "037fb52a7e8c89151fc6460513f585c8b82cd70a363003bc33a341003fc3a412b2"

	privateKey2 = "42050e738eedd791d2d827b443ab3aee2ff893add73f0c0187ce973ba46f9603"
	publicKey2  = "02fbe91249dc684114381c1cc9e0d8edb92f962175318b1d08aea32c99fcd246a9"

	msg             = "hello world"
	encryptedSource = "975af04e7a7a1fdd290175064b48ffca47ae36c72678bd064e67bf19b91b6790087012173e427f"
	sskSource       = "f6a9bb98468506b517d9d9e9ad31b699f9afb6f05599760a68f1a39758f529e1"
)

func TestGenerateDHSessionKey(t *testing.T) {
	ssk, err := GenerateDHSessionKey(privateKey1, publicKey2)
	if err != nil {
		t.Fatalf("GenerateDHSessionKey failed: %v", err)
	}
	got := hex.EncodeToString(ssk)
	if got != sskSource {
		t.Fatalf("SSK mismatch:\n  got:  %s\n  want: %s", got, sskSource)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	encrypted, err := EncryptWithDHKeyPair(privateKey1, publicKey2, []byte(msg))
	if err != nil {
		t.Fatalf("EncryptWithDHKeyPair failed: %v", err)
	}
	decrypted, err := DecryptWithDHKeyPair(privateKey2, publicKey1, encrypted)
	if err != nil {
		t.Fatalf("DecryptWithDHKeyPair failed: %v", err)
	}
	if string(decrypted) != msg {
		t.Fatalf("round-trip mismatch: got %q want %q", string(decrypted), msg)
	}
}

func TestDecryptKnownCiphertext(t *testing.T) {
	ciphertext, err := hex.DecodeString(encryptedSource)
	if err != nil {
		t.Fatalf("hex decode ciphertext: %v", err)
	}
	decrypted, err := DecryptWithDHKeyPair(privateKey2, publicKey1, ciphertext)
	if err != nil {
		t.Fatalf("DecryptWithDHKeyPair failed: %v", err)
	}
	if string(decrypted) != msg {
		t.Fatalf("decrypt mismatch: got %q want %q", string(decrypted), msg)
	}
}
