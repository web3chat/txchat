package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/haltingstate/secp256k1-go"
)

type userQueryPayload struct {
	MainAddress   string        `json:"mainAddress"`
	TargetAddress string        `json:"targetAddress"`
	Count         int           `json:"count"`
	Index         string        `json:"index"`
	Time          int64         `json:"time"`
	Sign          querySign     `json:"sign"`
}

type querySign struct {
	PublicKey string `json:"publicKey"`
	Signature string `json:"signature"`
}

type userResult struct {
	Fields []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"fields"`
}

func signParams(params map[string]interface{}, privKeyBytes []byte) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, params[k]))
	}
	msg := strings.Join(parts, "&")

	hash := sha256.Sum256([]byte(msg))
	sig := secp256k1.Sign(hash[:], privKeyBytes)
	return hex.EncodeToString(sig)
}

// GetUserPubKey queries the chain for a user's public key.
// agentPrivKey is the 32-byte raw private key, agentPubKeyHex is the compressed public key hex.
func GetUserPubKey(chainRPC, agentAddr, targetAddr string, agentPrivKey []byte, agentPubKeyHex string) (string, error) {
	if chainRPC == "" {
		return "", fmt.Errorf("chain RPC not configured")
	}

	now := time.Now().UnixMilli()
	params := map[string]interface{}{
		"mainAddress":   agentAddr,
		"targetAddress": targetAddr,
		"count":         40,
		"index":         "",
		"time":          now,
	}
	sig := signParams(params, agentPrivKey)

	payload := userQueryPayload{
		MainAddress:   agentAddr,
		TargetAddress: targetAddr,
		Count:         40,
		Index:         "",
		Time:          now,
		Sign: querySign{
			PublicKey: agentPubKeyHex,
			Signature: sig,
		},
	}
	query := map[string]interface{}{
		"execer":   "chat",
		"funcName": "GetUser",
		"payload":  payload,
	}

	raw, err := Call(chainRPC, "Chain33.Query", query)
	if err != nil {
		return "", fmt.Errorf("chain query GetUser: %w", err)
	}

	var result userResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse GetUser result: %w raw=%s", err, string(raw))
	}

	for _, f := range result.Fields {
		if f.Name == "pubKey" && f.Value != "" {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("pubKey not found for address %s", targetAddr)
}
