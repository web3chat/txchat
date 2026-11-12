package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	aesNonceLength = 12
	sskLen         = 16
)

func ecdhSharedKey(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) ([]byte, error) {
	if privKey.Curve != pubKey.Curve {
		return nil, fmt.Errorf("curve mismatch")
	}
	x, _ := pubKey.Curve.ScalarMult(pubKey.X, pubKey.Y, privKey.D.Bytes())
	if x == nil {
		return nil, fmt.Errorf("shared key is point at infinity")
	}
	sk := make([]byte, sskLen+sskLen)
	xBytes := x.Bytes()
	copy(sk[len(sk)-len(xBytes):], xBytes)
	return sk, nil
}

func GenerateDHSessionKey(privateKeyHex, publicKeyHex string) ([]byte, error) {
	ecdsaPriv, err := ethcrypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	pubBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("decode public key hex: %w", err)
	}
	ecdsaPub, err := ethcrypto.DecompressPubkey(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("decompress public key: %w", err)
	}
	return ecdhSharedKey(ecdsaPriv, ecdsaPub)
}

func EncryptWithDHKeyPair(privateKeyHex, publicKeyHex string, plaintext []byte) ([]byte, error) {
	key, err := GenerateDHSessionKey(privateKeyHex, publicKeyHex)
	if err != nil {
		return nil, err
	}
	return encryptSymmetric(key, plaintext)
}

func DecryptWithDHKeyPair(privateKeyHex, publicKeyHex string, ciphertext []byte) ([]byte, error) {
	key, err := GenerateDHSessionKey(privateKeyHex, publicKeyHex)
	if err != nil {
		return nil, err
	}
	return decryptSymmetric(key, ciphertext)
}

func encryptSymmetric(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesNonceLength)
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	encrypted := aesgcm.Seal(nil, nonce, plaintext, nil)
	return append(encrypted, nonce...), nil
}

func decryptSymmetric(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aesNonceLength {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[len(ciphertext)-aesNonceLength:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aesgcm.Open(nil, nonce, ciphertext[:len(ciphertext)-aesNonceLength], nil)
}

func PrivKeyToHex(privKeyBytes []byte) string {
	return hex.EncodeToString(privKeyBytes)
}

func PubKeyFromPrivKey(privKeyBytes []byte) string {
	privKey, err := ethcrypto.ToECDSA(privKeyBytes)
	if err != nil {
		return ""
	}
	pubBytes := elliptic.MarshalCompressed(ethcrypto.S256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	return hex.EncodeToString(pubBytes)
}

func PubKeyToAddress(publicKeyHex string) (string, error) {
	pubBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return "", err
	}
	ecdsaPub, err := ethcrypto.DecompressPubkey(pubBytes)
	if err != nil {
		return "", err
	}
	addr := ethcrypto.PubkeyToAddress(*ecdsaPub)
	return addr.Hex(), nil
}

