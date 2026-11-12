package service

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestPersonalSignHashUsesEthereumPrefix(t *testing.T) {
	t.Parallel()
	message := []byte("hello")
	prefixed := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message))
	want := crypto.Keccak256(prefixed)
	got := personalSignHash(message)
	if !bytes.Equal(got, want) {
		t.Fatalf("hash = %s, want %s", hexutil.Encode(got), hexutil.Encode(want))
	}
}

func TestIsEthAddress(t *testing.T) {
	t.Parallel()
	cases := []struct {
		addr string
		ok   bool
	}{
		{addr: "0xabcdeffabcdeffabcdeffabcdeffabcdeffabcdf", ok: true},
		{addr: "0xABCDEFFABCDEFFABCDEFFABCDEFFABCDEFFABCDF", ok: false},
		{addr: "abcdeffabcdeffabcdeffabcdeffabcdeffabcdf", ok: false},
		{addr: "0xdeadbeef", ok: false},
		{addr: "", ok: false},
	}
	for _, tc := range cases {
		if got := isEthAddress(tc.addr); got != tc.ok {
			t.Errorf("isEthAddress(%q)=%v, want %v", tc.addr, got, tc.ok)
		}
	}
}

func TestRecoverPersonalSignAddress(t *testing.T) {
	t.Parallel()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	want := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	nonce := "DTalk Admin Login:1730000000000000000"
	sig, err := crypto.Sign(personalSignHash([]byte(nonce)), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig[64] += 27
	encoded := hexutil.Encode(sig)

	got, err := recoverPersonalSignAddress(nonce, encoded)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if got != want {
		t.Fatalf("recovered %s, want %s", got, want)
	}

	other, err := recoverPersonalSignAddress("other message", encoded)
	if err != nil {
		t.Fatalf("recover other message: %v", err)
	}
	if other == want {
		t.Fatal("signature of another message recovered the same address")
	}

	if _, err := recoverPersonalSignAddress(nonce, "0xdeadbeef"); err == nil {
		t.Fatal("short signature was accepted")
	}

	raw := make([]byte, 65)
	copy(raw, sig)
	raw[0] ^= 0xff
	recovered, err := recoverPersonalSignAddress(nonce, hexutil.Encode(raw))
	if err == nil && recovered == want {
		t.Fatal("tampered signature recovered the signer")
	}
}
