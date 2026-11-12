package service

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// isEthAddress 判断小写后的地址是否为 0x 加 40 位十六进制。
func isEthAddress(addr string) bool {
	if len(addr) != 42 || !strings.HasPrefix(addr, "0x") {
		return false
	}
	for _, c := range addr[2:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// personalSignHash 计算 eth_sign / personal_sign 的消息哈希。
// 前缀为 "\x19Ethereum Signed Message:\n" 加上消息字节长度。
func personalSignHash(message []byte) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	return crypto.Keccak256(append([]byte(prefix), message...))
}

// recoverPersonalSignAddress 从 personal_sign 签名恢复签名者地址。
// 消息必须与下发给钱包的 nonce 原文一致。签名为 0x 开头的 65 字节，末字节 27/28 会先换成 0/1。
func recoverPersonalSignAddress(message, signature string) (string, error) {
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}
	if len(sig) != 65 {
		return "", fmt.Errorf("signature length %d", len(sig))
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if sig[64] != 0 && sig[64] != 1 {
		return "", fmt.Errorf("invalid recovery id %d", sig[64])
	}
	pub, err := crypto.SigToPub(personalSignHash([]byte(message)), sig)
	if err != nil {
		return "", fmt.Errorf("recover pubkey: %w", err)
	}
	return strings.ToLower(crypto.PubkeyToAddress(*pub).Hex()), nil
}
