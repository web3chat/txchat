package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec"
	"github.com/web3chat/txchat/dtalk/pkg/address"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	var (
		username   string
		password   string
		name       string
		avatar     string
		sortOrder  int
		aesKey     string
		privateKey string
		genKey     bool
	)

	flag.StringVar(&username, "username", "", "登录用户名（必填）")
	flag.StringVar(&password, "password", "", "登录密码（必填）")
	flag.StringVar(&name, "name", "", "客服显示名称（必填）")
	flag.StringVar(&avatar, "avatar", "", "头像 URL（可选）")
	flag.IntVar(&sortOrder, "sort", 0, "展示排序（可选，默认0）")
	flag.StringVar(&aesKey, "aes-key", "", "AES-256 加密密钥，32字节hex（64字符）。不提供则明文存储私钥")
	flag.StringVar(&privateKey, "private-key", "", "secp256k1 私钥 hex（64字符）。不提供则自动生成")
	flag.BoolVar(&genKey, "gen-key", false, "仅生成一个新的密钥对并打印，不生成 SQL")
	flag.Parse()

	if genKey {
		doGenKey()
		return
	}

	// 交互模式：如果必填参数缺失，进入交互式输入
	reader := bufio.NewReader(os.Stdin)
	if username == "" {
		username = prompt(reader, "请输入登录用户名")
	}
	if password == "" {
		password = prompt(reader, "请输入登录密码")
	}
	if name == "" {
		name = prompt(reader, "请输入客服显示名称")
	}
	if privateKey == "" {
		fmt.Println("\n未指定私钥，自动生成新的 secp256k1 密钥对...")
	}

	if username == "" || password == "" || name == "" {
		fmt.Fprintln(os.Stderr, "错误: username, password, name 为必填项")
		os.Exit(1)
	}

	// 1. 获取或生成密钥对
	privKeyBytes, pubKeyBytes := getOrGenerateKeyPair(privateKey)
	privKeyHex := hex.EncodeToString(privKeyBytes)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	agentID := address.PublicKeyToAddress(address.NormalVer, pubKeyBytes)

	// 2. bcrypt 密码哈希
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fatal("bcrypt 哈希失败: %v", err)
	}
	passwordHash := string(hash)

	// 3. 私钥加密存储
	var privateKeyEnc string
	if aesKey != "" {
		aesKeyBytes, err := hex.DecodeString(aesKey)
		if err != nil || len(aesKeyBytes) != 32 {
			fatal("AES 密钥格式无效，需要64位十六进制字符串（32字节）")
		}
		encrypted, err := aesEncrypt([]byte(privKeyHex), aesKeyBytes)
		if err != nil {
			fatal("AES 加密失败: %v", err)
		}
		privateKeyEnc = hex.EncodeToString(encrypted)
	} else {
		privateKeyEnc = privKeyHex
		fmt.Println("\n⚠ 警告: 未提供 AES 密钥，私钥将以明文存储。生产环境请使用 -aes-key 参数")
	}

	// 4. 输出信息
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("  客服账号信息")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("  用户名:           %s\n", username)
	fmt.Printf("  显示名称:         %s\n", name)
	fmt.Printf("  IM 地址:          %s\n", agentID)
	fmt.Printf("  公钥:             %s\n", pubKeyHex)
	if privateKey == "" {
		fmt.Printf("  私钥(请妥善保存): %s\n", privKeyHex)
	}
	fmt.Println(strings.Repeat("=", 70))

	// 5. 输出 SQL
	fmt.Println("\n-- 复制以下 SQL 插入到数据库:")
	fmt.Printf(`INSERT INTO cs_agent (agent_id, username, password_hash, private_key_enc, public_key, name, avatar, status, sort_order)
VALUES ('%s', '%s', '%s', '%s', '%s', '%s', '%s', 0, %d);
`,
		escapeSQLString(agentID),
		escapeSQLString(username),
		escapeSQLString(passwordHash),
		escapeSQLString(privateKeyEnc),
		escapeSQLString(pubKeyHex),
		escapeSQLString(name),
		escapeSQLString(avatar),
		sortOrder,
	)
}

func getOrGenerateKeyPair(privateKeyHex string) (privKey, pubKey []byte) {
	if privateKeyHex != "" {
		privBytes, err := hex.DecodeString(privateKeyHex)
		if err != nil || len(privBytes) != 32 {
			fatal("私钥格式无效，需要64位十六进制字符串（32字节）")
		}
		privKeyObj, pubKeyObj := btcec.PrivKeyFromBytes(btcec.S256(), privBytes)
		return privKeyObj.Serialize(), pubKeyObj.SerializeCompressed()
	}

	privKeyObj, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fatal("生成密钥对失败: %v", err)
	}
	return privKeyObj.Serialize(), privKeyObj.PubKey().SerializeCompressed()
}

func doGenKey() {
	privKeyObj, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fatal("生成密钥对失败: %v", err)
	}
	privKeyBytes := privKeyObj.Serialize()
	pubKeyBytes := privKeyObj.PubKey().SerializeCompressed()

	privKeyHex := hex.EncodeToString(privKeyBytes)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	agentID := address.PublicKeyToAddress(address.NormalVer, pubKeyBytes)

	fmt.Println("新生成的 secp256k1 密钥对:")
	fmt.Printf("  私钥: %s\n", privKeyHex)
	fmt.Printf("  公钥: %s\n", pubKeyHex)
	fmt.Printf("  地址: %s\n", agentID)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Printf("%s: ", label)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func escapeSQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func aesEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return aesGCM.Seal(nonce, nonce, plaintext, nil), nil
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}
