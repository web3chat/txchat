package auth

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoldenExpiredTokenFromTestdata(t *testing.T) {
	raw, err := os.ReadFile("testdata/expired.token")
	assert.Nil(t, err)
	sig := strings.TrimSpace(string(raw))
	server := NewDefaultApiAuthenticator()
	uid, err := server.Auth(sig)
	assert.ErrorIs(t, err, ERR_SIGNATUREEXPIRED)
	assert.Empty(t, uid)
}

func TestGoldenFreshTokenFromFixedKey(t *testing.T) {
	pubKey, err := hex.DecodeString(publicKey)
	assert.Nil(t, err)
	privKey, err := hex.DecodeString(privateKey)
	assert.Nil(t, err)
	client := NewDefaultApiAuthenticator()
	sig := client.Request("dtalk", pubKey, privKey)
	uid, err := NewDefaultApiAuthenticator().Auth(sig)
	assert.Nil(t, err)
	assert.Equal(t, userAddress, uid)
}
