// Package chatclean 提供针对聊天记录清理功能的端到端接口测试。
//
// 测试向「对外 HTTP 入口」发起请求（生产/联调场景下通常为 nginx 反代到 gateway），
// 而不是直连 gateway 监听端口；默认 base URL 与线上一致可调。
//
// 使用前提：
//  1. nginx（或等价入口）已将路径转发到 gateway，且 gateway 已就绪
//  2. MySQL 中已执行 script/mysql/dtalk_chatclean.sql
//  3. 默认使用文件内固定的测试私钥生成 FZM-SIGNATURE（与 pkg/auth 一致）；服务端需识别对应公钥/用户
//
// 运行：
//
//	# 默认使用 defaultGatewayBase（nginx 入口），固定测试私钥签名
//	go test ./test/chatclean/... -v
//	go test ./test/chatclean/... -v -run TestCleanPrivateChat
//
//	# 指定入口（仍为 nginx/对外域名，非直连 gateway 端口时同理）
//	DTALK_GATEWAY_URL=https://your-api.example.com go test ./test/chatclean/... -v
//
//	# 不依赖 .env，直接通过测试参数传助记词（参数优先于环境变量）
//	go test ./test/chatclean/... -v -args \
//	  -test-mnemonic-a "word1 ... word12" -test-derivation-path-a "m/44'/60'/0'/0/0" \
//	  -test-mnemonic-b "word1 ... word12" -test-derivation-path-b "m/44'/60'/0'/0/1"
//
// 可选环境变量（覆盖默认值）：
//
//	DTALK_GATEWAY_URL — 对外 base URL（默认见 defaultGatewayBase）
//	DTALK_TEST_PRIVKEY_A / DTALK_TEST_PRIVKEY_B — 覆盖内置测试私钥（hex，可带 0x）
//	DTALK_TEST_PUBKEY_A / DTALK_TEST_PUBKEY_B — 可选；不填则从私钥推导压缩公钥
//	DTALK_TEST_MNEMONIC_A / DTALK_TEST_MNEMONIC_B — 助记词派生私钥（优先级低于显式私钥）
//	DTALK_TEST_DERIVATION_PATH_A / DTALK_TEST_DERIVATION_PATH_B — 助记词派生路径（默认 m/44'/60'/0'/0/0 与 .../1）
//	DTALK_TEST_MNEMONIC_PASSPHRASE_A / DTALK_TEST_MNEMONIC_PASSPHRASE_B — 助记词可选密码
//	DTALK_TEST_APPID — 默认 dtalk
package chatclean

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/web3chat/txchat/dtalk/pkg/address"
	"github.com/web3chat/txchat/dtalk/pkg/auth"
	comet "github.com/web3chat/txchat/im/api/comet/grpc"
	xproto "github.com/web3chat/txchat/imparse/proto"
	"github.com/tyler-smith/go-bip39"
)

// ─── 配置 ────────────────────────────────────────────────────────────────────

const (
	// defaultGatewayBase：对外 HTTP 入口（示例为经 nginx 暴露的端口），非 gateway 进程直连地址。
	defaultGatewayBase = "http://localhost:8888"

	// 内置 secp256k1 测试私钥（Hardhat 常用本地账户 #0 / #1），仅用于集成测试；可按需用环境变量覆盖。
	defaultPrivKeyHexA = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	defaultPrivKeyHexB = "31f71b55ea7950128d9673fa25a3eafdfe348791b21007bf70082bd8078ed84a"

	// 逻辑用户别名
	userA = "MOCK"
	userB = "MOCK2"
)

// ─── 设备与 UUID 配置 ──────────────────────────────────────────────────────────

const (
	uuidA         = "a2abfc63-e3ec-4230-8177-4989e20ba2a7"
	uuidB         = "b2abfc63-e3ec-4230-8177-4989e20ba2a7"
	fzmDevice     = "Android"
	fzmDeviceName = "TestDevice"
	fzmVersion    = "1.0.0-test"
)

// getUserUUID 根据用户标识返回对应的设备 UUID。
func getUserUUID(user string) string {
	switch user {
	case userA:
		return uuidA
	case userB:
		return uuidB
	default:
		return "00000000-0000-0000-0000-000000000000"
	}
}

// envGatewayBase 返回对外 API base URL（通常为 nginx）：优先 DTALK_GATEWAY_URL，否则 defaultGatewayBase。
func envGatewayBase() string {
	if u := strings.TrimSpace(os.Getenv("DTALK_GATEWAY_URL")); u != "" {
		return strings.TrimSuffix(u, "/")
	}
	return defaultGatewayBase
}

// ─── 非 MOCK 鉴权（与 pkg/auth 一致：用私钥生成 FZM-SIGNATURE）──────────────────

var (
	realAuthOnce sync.Once
	realAuth     *authHelper

	// go test -args 参数方式（不依赖 .env），优先于同名环境变量。
	testMnemonicAFlag           = flag.String("test-mnemonic-a", "", "mnemonic words for user A")
	testMnemonicBFlag           = flag.String("test-mnemonic-b", "", "mnemonic words for user B")
	testDerivationPathAFlag     = flag.String("test-derivation-path-a", "", "derivation path for user A, e.g. m/44'/60'/0'/0/0")
	testDerivationPathBFlag     = flag.String("test-derivation-path-b", "", "derivation path for user B, e.g. m/44'/60'/0'/0/1")
	testMnemonicPassphraseAFlag = flag.String("test-mnemonic-passphrase-a", "", "mnemonic passphrase for user A")
	testMnemonicPassphraseBFlag = flag.String("test-mnemonic-passphrase-b", "", "mnemonic passphrase for user B")
)

type authHelper struct {
	authenticator auth.ApiAuthenticator
	appID         string
	privA         []byte
	privB         []byte
	pubA          []byte
	pubB          []byte
	useRealA      bool
	useRealB      bool
}

// ─── 测试用例 ─────────────────────────────────────────────────────────────────
func TestLogin(t *testing.T) {
	// 保证每次执行该用例都按当前环境变量重新初始化鉴权配置。
	realAuth = nil
	realAuthOnce = sync.Once{}

	//t.Logf("derived address(A)=%s", addressOfUser(userA))
	//t.Logf("derived address(B)=%s", addressOfUser(userB))

	addrA := doLogin(t, userA)
	t.Logf("login address(A)=%s", addrA)
}

func initRealAuth() *authHelper {
	h := &authHelper{appID: "dtalk"}
	if v := strings.TrimSpace(os.Getenv("DTALK_TEST_APPID")); v != "" {
		h.appID = v
	}
	//*testMnemonicAFlag = "士 宋 水 私 撑 钙 冰 样 锐 主 紫 氧 挡 前 型"
	//*testMnemonicAFlag = "wheel scale fence enrich hint nerve embrace march culture inquiry bullet invest"
	//*testMnemonicAFlag = "option jeans mercy cheap inflict edit rival melody nest quarter two bomb"
	//*testMnemonicAFlag = "echo achieve hidden assume two outside hole ranch below shuffle witness emotion"
	privAHex := strings.TrimSpace(os.Getenv("DTALK_TEST_PRIVKEY_A"))
	privBHex := strings.TrimSpace(os.Getenv("DTALK_TEST_PRIVKEY_B"))
	mnemonicA := firstNonEmpty(strings.TrimSpace(*testMnemonicAFlag), strings.TrimSpace(os.Getenv("DTALK_TEST_MNEMONIC_A")))
	mnemonicB := firstNonEmpty(strings.TrimSpace(*testMnemonicBFlag), strings.TrimSpace(os.Getenv("DTALK_TEST_MNEMONIC_B")))
	pathA := firstNonEmpty(strings.TrimSpace(*testDerivationPathAFlag), strings.TrimSpace(os.Getenv("DTALK_TEST_DERIVATION_PATH_A")))
	pathB := firstNonEmpty(strings.TrimSpace(*testDerivationPathBFlag), strings.TrimSpace(os.Getenv("DTALK_TEST_DERIVATION_PATH_B")))
	passA := firstNonEmpty(strings.TrimSpace(*testMnemonicPassphraseAFlag), strings.TrimSpace(os.Getenv("DTALK_TEST_MNEMONIC_PASSPHRASE_A")))
	passB := firstNonEmpty(strings.TrimSpace(*testMnemonicPassphraseBFlag), strings.TrimSpace(os.Getenv("DTALK_TEST_MNEMONIC_PASSPHRASE_B")))
	if pathA == "" {
		pathA = "m/44'/60'/0'/0/0"
	}
	if pathB == "" {
		pathB = "m/44'/60'/0'/0/1"
	}
	fmt.Println("mnemonic A:", mnemonicA)
	h.authenticator = auth.NewDefaultApiAuthenticator()
	parseKey := func(privHex, pubKeyEnv string) (priv, pub []byte, ok bool) {
		normalizedHex := strings.TrimPrefix(strings.TrimSpace(privHex), "0x")
		priv, err := hex.DecodeString(normalizedHex)
		if err != nil || len(priv) != 32 {
			return nil, nil, false
		}
		pubHex := strings.TrimSpace(os.Getenv(pubKeyEnv))
		if pubHex != "" {
			pub, err = hex.DecodeString(strings.TrimPrefix(pubHex, "0x"))
			if err != nil || len(pub) != 33 {
				return nil, nil, false
			}
			return priv, pub, true
		}
		ecdsaPriv, err := crypto.HexToECDSA(normalizedHex)
		if err != nil {
			return nil, nil, false
		}
		pub = crypto.CompressPubkey(&ecdsaPriv.PublicKey)
		return priv, pub, true
	}

	deriveByMnemonic := func(mnemonic, passphrase, derivationPath string) (priv, pub []byte, err error) {
		if mnemonic == "" {
			return nil, nil, fmt.Errorf("empty mnemonic")
		}
		// 兼容非英文词表助记词：若校验失败仍尝试按 BIP39 seed 规则派生。
		if !bip39.IsMnemonicValid(mnemonic) {
			fmt.Printf("[chatclean] mnemonic validation failed, continue with raw seed derivation\n")
		}
		seed := bip39.NewSeed(mnemonic, passphrase)
		key, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
		if err != nil {
			return nil, nil, fmt.Errorf("new master key: %w", err)
		}
		for _, idx := range parseDerivationPath(derivationPath) {
			key, err = deriveHDChild(key, idx)
			if err != nil {
				return nil, nil, fmt.Errorf("derive child %d: %w", idx, err)
			}
		}
		k, err := key.ECPrivKey()
		if err != nil {
			return nil, nil, fmt.Errorf("extract EC private key: %w", err)
		}
		priv = k.Serialize()
		ecdsaPriv, err := crypto.ToECDSA(priv)
		if err != nil {
			return nil, nil, fmt.Errorf("to ECDSA: %w", err)
		}
		pub = crypto.CompressPubkey(&ecdsaPriv.PublicKey)
		return priv, pub, nil
	}

	resolveUserKey := func(explicitPrivHex, defaultPrivHex, mnemonic, passphrase, derivationPath, pubKeyEnv string) (priv, pub []byte, ok bool) {
		if explicitPrivHex != "" {
			fmt.Printf("[chatclean] auth source=%s explicit private key\n", pubKeyEnv)
			return parseKey(explicitPrivHex, pubKeyEnv)
		}
		if mnemonic != "" {

			cleanPath := normalizeDerivationPath(derivationPath)

			priv, pub, err := deriveByMnemonic(mnemonic, passphrase, cleanPath)
			if err == nil {
				fmt.Printf("[chatclean] auth source=%s mnemonic path=%s address=%s\n",
					pubKeyEnv, cleanPath, address.PublicKeyToAddress(address.NormalVer, pub))
				return priv, pub, true
			}
			fmt.Printf("[chatclean] auth source=%s mnemonic derive failed (path=%s, err=%v), fallback default private key\n",
				pubKeyEnv, cleanPath, err)
			return parseKey(defaultPrivHex, pubKeyEnv)
		}
		fmt.Printf("[chatclean] auth source=%s built-in default private key\n", pubKeyEnv)
		return parseKey(defaultPrivHex, pubKeyEnv)
	}

	if privA, pubA, ok := resolveUserKey(privAHex, defaultPrivKeyHexA, mnemonicA, passA, pathA, "DTALK_TEST_PUBKEY_A"); ok {
		h.privA, h.pubA, h.useRealA = privA, pubA, true
	}
	if privB, pubB, ok := resolveUserKey(privBHex, defaultPrivKeyHexB, mnemonicB, passB, pathB, "DTALK_TEST_PUBKEY_B"); ok {
		h.privB, h.pubB, h.useRealB = privB, pubB, true
	}
	if !h.useRealA && !h.useRealB {
		return nil
	}
	return h
}

func parseDerivationPath(path string) []uint32 {
	const hardenedOffset = uint32(0x80000000)
	defaultPath := []uint32{44 + hardenedOffset, 60 + hardenedOffset, 0 + hardenedOffset, 0, 0}

	p := normalizeDerivationPath(path)
	if p == "" {
		return defaultPath
	}
	parts := strings.Split(p, "/")
	if len(parts) == 0 {
		return defaultPath
	}
	out := make([]uint32, 0, len(parts))
	for i, seg := range parts {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if i == 0 && (seg == "m" || seg == "M") {
			continue
		}
		hardened := strings.HasSuffix(seg, "'")
		if hardened {
			seg = strings.TrimSuffix(seg, "'")
		}
		n, err := strconv.ParseUint(seg, 10, 32)
		if err != nil {
			return defaultPath
		}
		if hardened {
			n += uint64(hardenedOffset)
		}
		if n > math.MaxUint32 {
			return defaultPath
		}
		out = append(out, uint32(n))
	}
	if len(out) == 0 {
		return defaultPath
	}
	return out
}

func normalizeDerivationPath(path string) string {
	p := strings.TrimSpace(path)
	// 兼容配置里误带的包裹引号。
	p = strings.Trim(p, "\"'")
	p = strings.Trim(p, "“”‘’")
	return strings.TrimSpace(p)
}

// deriveHDChild 兼容不同版本 hdkeychain API：
// 新版本常用 Derive，旧版本常用 Child。
func deriveHDChild(key *hdkeychain.ExtendedKey, idx uint32) (*hdkeychain.ExtendedKey, error) {
	call := func(method string) (*hdkeychain.ExtendedKey, error, bool) {
		m := reflect.ValueOf(key).MethodByName(method)
		if !m.IsValid() {
			return nil, nil, false
		}
		out := m.Call([]reflect.Value{reflect.ValueOf(idx)})
		if len(out) != 2 {
			return nil, fmt.Errorf("%s returned unexpected values", method), true
		}
		next, _ := out[0].Interface().(*hdkeychain.ExtendedKey)
		var err error
		if !out[1].IsNil() {
			err, _ = out[1].Interface().(error)
		}
		return next, err, true
	}
	if next, err, ok := call("Derive"); ok {
		return next, err
	}
	if next, err, ok := call("Child"); ok {
		return next, err
	}
	return nil, fmt.Errorf("hdkeychain has neither Derive nor Child")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// getRealAuth 返回鉴权助手（懒加载；支持显式私钥或助记词派生，未配置时回退内置测试私钥）。
func getRealAuth() *authHelper {
	realAuthOnce.Do(func() {
		realAuth = initRealAuth()
	})
	return realAuth
}

// addressOfUser 从公钥推导区块链地址，用于 TargetID 等场景。
func addressOfUser(user string) string {
	h := getRealAuth()
	if h == nil {
		return user
	}
	switch user {
	case userA:
		if h.useRealA {
			return address.PublicKeyToAddress(address.NormalVer, h.pubA)
		}
	case userB:
		if h.useRealB {
			return address.PublicKeyToAddress(address.NormalVer, h.pubB)
		}
	}
	return user
}

// getSignature 返回用于 FZM-SIGNATURE 的值：非 MOCK 模式下用 pkg/auth 生成 token，否则返回 MOCK/MOCK2。
func getSignature(user string) string {
	h := getRealAuth()
	if h == nil {
		return user
	}
	gen := func(priv, pub []byte) string {
		if len(priv) == 0 || len(pub) == 0 {
			return ""
		}
		return h.authenticator.Request(h.appID, pub, priv)
	}
	if user == userA && h.useRealA {
		if sig := gen(h.privA, h.pubA); sig != "" {
			return sig
		}
	}
	if user == userB && h.useRealB {
		if sig := gen(h.privB, h.pubB); sig != "" {
			return sig
		}
	}
	return user
}

// ─── 请求/响应类型（与 gateway model 保持一致）───────────────────────────────

type GeneralResponse struct {
	Result  int             `json:"result"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type AddressLoginResp struct {
	Address string `json:"address"`
}

type CleanPrivateChatReq struct {
	TargetID  string `json:"targetId"`
	DateOpt   int32  `json:"dateOpt"`
	StartTime int64  `json:"startTime,omitempty"`
	EndTime   int64  `json:"endTime,omitempty"`
}

type CleanPrivateChatResp struct {
	DeletedCount int64 `json:"deletedCount"`
}

type SetAutoDeleteConfigReq struct {
	TargetID string `json:"targetId"`
	Option   int8   `json:"option"`
}

type AutoDeleteConfigResp struct {
	Option    int8   `json:"option"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedAt int64  `json:"updatedAt"`
}

type GetAutoDeleteConfigReq struct {
	TargetID string `json:"targetId"`
}

type GetPriRecordsReq struct {
	TargetID string `json:"targetId"`
	Count    int64  `json:"count"`
	LogID    string `json:"logId,omitempty"`
}

type PriRecord struct {
	LogID      string          `json:"logId"`
	MsgID      string          `json:"msgId"`
	FromID     string          `json:"fromId"`
	TargetID   string          `json:"targetId"`
	MsgType    int32           `json:"msgType"`
	Content    json.RawMessage `json:"content"`
	CreateTime uint64          `json:"createTime"`
}

type GetPriRecordsResp struct {
	RecordCount int         `json:"record_count"`
	Records     []PriRecord `json:"records"`
}

type SyncRecordsReq struct {
	Count int64 `json:"count"`
	Start int64 `json:"start"`
}

type SyncRecordsResp struct {
	RecordCount int      `json:"record_count"`
	Records     []string `json:"records"`
}

// ─── 工具函数 ─────────────────────────────────────────────────────────────────

// doPost 发送 POST 请求，带 FZM-SIGNATURE / FZM-UUID / FZM-DEVICE 完整请求头。
func doPost(t *testing.T, path string, user string, body interface{}) *GeneralResponse {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, envGatewayBase()+path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	signature := getSignature(user)
	req.Header.Set("FZM-SIGNATURE", signature)
	req.Header.Set("FZM-UUID", getUserUUID(user))
	req.Header.Set("FZM-DEVICE", fzmDevice)
	req.Header.Set("FZM-DEVICE-NAME", fzmDeviceName)
	req.Header.Set("FZM-VERSION", fzmVersion)

	t.Logf("→ POST %s (user=%s, uuid=%s)", path, user, getUserUUID(user))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	t.Logf("← %d  ", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected HTTP status %d, body=%s", resp.StatusCode, b)
	}

	var gr GeneralResponse
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&gr); err != nil {
		t.Fatalf("decode response: %v (raw=%s)", err, b)
	}
	return &gr
}

// doLogin 对指定用户执行 /user/login 认证，返回服务端确认的用户地址。
func doLogin(t *testing.T, user string) string {
	t.Helper()
	gr := doPost(t, "/user/login", user, map[string]interface{}{"connType": 0})
	assertOK(t, gr)

	var resp AddressLoginResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal login resp: %v", err)
	}
	t.Logf("✔ %s logged in, address=%s", user, resp.Address)
	return resp.Address
}

// buildTextMessageBody 构造符合 imparse 协议的文本消息 body（用于 /record/push 的 message 字段）。
func buildTextMessageBody(from, target, content string) ([]byte, error) {
	textMsg := &xproto.TextMsg{Content: content}
	textBytes, err := proto.Marshal(textMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal TextMsg: %w", err)
	}

	common := &xproto.Common{
		ChannelType: xproto.Channel_ToUser,
		Seq:         uuid.New().String(),
		From:        from,
		Target:      target,
		MsgType:     xproto.MsgType_Text,
		Msg:         textBytes,
		Datetime:    uint64(time.Now().UnixMilli()),
	}
	commonBytes, err := proto.Marshal(common)
	if err != nil {
		return nil, fmt.Errorf("marshal Common: %w", err)
	}

	imProto := &xproto.Proto{
		EventType: xproto.Proto_common,
		Body:      commonBytes,
	}
	return proto.Marshal(imProto)
}

// doPushMessage 通过 /record/push 发送一条文本消息。
// push 接口使用 multipart/form-data 上传 message 文件，内容为 imparse 协议 body。
// 返回响应（包含 logId, datetime）。
func doPushMessage(t *testing.T, fromUser, fromAddr, targetAddr, content string) *GeneralResponse {
	t.Helper()

	msgBody, err := buildTextMessageBody(fromAddr, targetAddr, content)
	if err != nil {
		t.Fatalf("build message body: %v", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("message", "msg.bin")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(msgBody); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, envGatewayBase()+"/record/push", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	signature := getSignature(fromUser)
	req.Header.Set("FZM-SIGNATURE", signature)
	req.Header.Set("FZM-UUID", getUserUUID(fromUser))
	req.Header.Set("FZM-DEVICE", fzmDevice)
	req.Header.Set("FZM-DEVICE-NAME", fzmDeviceName)
	req.Header.Set("FZM-VERSION", fzmVersion)

	t.Logf("→ PUSH from=%s target=%s content=%q", fromAddr, targetAddr, content)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	t.Logf("← %d %s", resp.StatusCode, b)

	var gr GeneralResponse
	if err := json.Unmarshal(b, &gr); err != nil {
		t.Fatalf("unmarshal push response: %v (raw=%s)", err, b)
	}
	return &gr
}

// doGetPriRecords 通过 /store/app/pri-chat-record 拉取私聊历史记录。
// 注意：该接口是发送方视角，返回 from=当前登录用户、target=请求 targetId 的记录。
func doGetPriRecords(t *testing.T, user, targetAddr string, count int64) *GetPriRecordsResp {
	t.Helper()
	gr := doPost(t, "/store/app/pri-chat-record", user, GetPriRecordsReq{
		TargetID: targetAddr,
		Count:    count,
	})
	assertOK(t, gr)

	var resp GetPriRecordsResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal pri records resp: %v, raw=%s", err, string(gr.Data))
	}
	return &resp
}

func doSyncRecords(t *testing.T, user string, start int64, count int64) *SyncRecordsResp {
	t.Helper()
	gr := doPost(t, "/app/record/sync-record", user, SyncRecordsReq{
		Count: count,
		Start: start,
	})
	assertOK(t, gr)

	var resp SyncRecordsResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal sync records resp: %v, raw=%s", err, string(gr.Data))
	}
	return &resp
}

func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &text); err == nil && text.Content != "" {
		return text.Content
	}

	// 兼容不同 JSON 序列化字段命名。
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m["content"].(string); ok {
		return v
	}
	if v, ok := m["Content"].(string); ok {
		return v
	}
	return ""
}

// formatCreateTimeUTC8 将聊天记录中的 createTime（毫秒 Unix 时间戳）格式化为东八区可读时间。
func formatCreateTimeUTC8(ms uint64) string {
	if ms == 0 {
		return "-"
	}
	loc := time.FixedZone("UTC+8", 8*3600)
	return time.UnixMilli(int64(ms)).In(loc).Format("2006-01-02 15:04:05.000") + " UTC+8"
}

// logPriRecordsDetail 在 go test -v 下打印私聊记录明细：解析出的文本 content、截断后的原始 JSON，以及 logId/msgId 等。
func logPriRecordsDetail(t *testing.T, title string, records []PriRecord) {
	t.Helper()
	t.Logf("──────── %s（%d 条）────────", title, len(records))
	for i, r := range records {
		text := extractTextContent(r.Content)
		raw := strings.TrimSpace(string(r.Content))
		if len(raw) > 320 {
			raw = raw[:320] + "…"
		}
		if raw == "" {
			raw = "(empty)"
		}
		t.Logf("  [%02d] logId=%s msgId=%s msgType=%d createTime=%d [%s] from=%s target=%s | text=%q | raw=%s",
			i+1, r.LogID, r.MsgID, r.MsgType, r.CreateTime, formatCreateTimeUTC8(r.CreateTime), r.FromID, r.TargetID, text, raw)
	}
}

func countRoundMessages(records []PriRecord, fromAddr, targetAddr, marker string) int {
	count := 0
	for _, r := range records {
		if r.FromID != fromAddr || r.TargetID != targetAddr {
			continue
		}
		if strings.Contains(extractTextContent(r.Content), marker) {
			count++
		}
	}
	return count
}

func parseSyncTextRecord(rawBase64 string) (from, target, content string, ok bool) {
	raw, err := base64.StdEncoding.DecodeString(rawBase64)
	if err != nil {
		return "", "", "", false
	}

	// sync-record 返回可能是 comet.Proto（外层）或业务 xproto.Proto（内层），两种都兼容。
	bizRaw := raw
	var cp comet.Proto
	if err := proto.Unmarshal(raw, &cp); err == nil && len(cp.Body) > 0 {
		bizRaw = cp.Body
	}

	var biz xproto.Proto
	if err := proto.Unmarshal(bizRaw, &biz); err != nil || biz.EventType != xproto.Proto_common {
		return "", "", "", false
	}

	var common xproto.Common
	if err := proto.Unmarshal(biz.Body, &common); err != nil {
		return "", "", "", false
	}
	if common.MsgType != xproto.MsgType_Text {
		return common.From, common.Target, "", true
	}

	var textMsg xproto.TextMsg
	if err := proto.Unmarshal(common.Msg, &textMsg); err != nil {
		return common.From, common.Target, "", true
	}
	return common.From, common.Target, textMsg.Content, true
}

func countSyncedRoundMessages(records []string, fromAddr, targetAddr, marker string) int {
	count := 0
	for _, r := range records {
		from, target, content, ok := parseSyncTextRecord(r)
		if !ok {
			continue
		}
		if from != fromAddr || target != targetAddr {
			continue
		}
		if strings.Contains(content, marker) {
			count++
		}
	}
	return count
}

func collectSyncedRoundContents(records []string, fromAddr, targetAddr, marker string) []string {
	contents := make([]string, 0, len(records))
	for _, r := range records {
		from, target, content, ok := parseSyncTextRecord(r)
		if !ok {
			continue
		}
		if from != fromAddr || target != targetAddr {
			continue
		}
		if strings.Contains(content, marker) {
			contents = append(contents, content)
		}
	}
	return contents
}

// assertOK 断言响应的 result == 0。
func assertOK(t *testing.T, gr *GeneralResponse) {
	t.Helper()
	if gr.Result != 0 {
		t.Errorf("expected result=0, got result=%d message=%s", gr.Result, gr.Message)
	}
}

// formatTimestamp 生成 "年-月-日-时-分" 格式的时间戳字符串。
func formatTimestamp(ts time.Time) string {
	return fmt.Sprintf("%d-%d-%d-%02d-%02d", ts.Year(), int(ts.Month()), ts.Day(), ts.Hour(), ts.Minute())
}

// ─── 测试用例 ─────────────────────────────────────────────────────────────────
//func TestLogin(t *testing.T) {
//	addrA := doLogin(t, userA)
//	t.Log(addrA)
//}

// TestFullFlow_LoginAndSendMessages 完整流程：登录 → 发送消息 → 拉取并校验接收内容。
func TestFullFlow_LoginAndSendMessages(t *testing.T) {
	// ── 第一步：双方登录认证 ──
	addrA := doLogin(t, userA)
	addrB := doLogin(t, userB)
	t.Logf("addrA=%s, addrB=%s", addrA, addrB)

	const msgCount = 50
	roundTS := fmt.Sprintf("%s-%d", formatTimestamp(time.Now()), time.Now().UnixNano())
	const fetchCount int64 = 100
	const syncCount int64 = 1000

	// ── 第二步：A 给 B 发送 50 条消息 ──
	t.Run("A_SendTo_B", func(t *testing.T) {
		for i := 1; i <= msgCount; i++ {
			content := fmt.Sprintf("[FromA To B 第%03d条] %s", i, roundTS)
			gr := doPushMessage(t, userA, addrA, addrB, content)
			if gr.Result != 0 {
				t.Errorf("msg %03d push failed: result=%d message=%s", i, gr.Result, gr.Message)
			}
		}
		t.Logf("A → B: %d messages sent, timestamp=%s", msgCount, roundTS)
	})

	// ── 第三步：校验 A→B 本轮消息已落库（发送方视角）──
	t.Run("A_Query_A_To_B_Stored", func(t *testing.T) {
		// pri-chat-record 为发送方视角，这里用 A 账号查询 A->B 方向记录来验证本轮发送已落库。
		resp := doGetPriRecords(t, userA, addrB, fetchCount)
		got := countRoundMessages(resp.Records, addrA, addrB, roundTS)
		if got < msgCount {
			t.Fatalf("A->B stored records not enough: want >= %d, got %d (record_count=%d)", msgCount, got, resp.RecordCount)
		}
		t.Logf("A->B stored records matched=%d, record_count=%d", got, resp.RecordCount)
	})

	// ── 第四步：B 通过 sync-record 拉取并校验收到了 A→B 本轮消息 ──
	t.Run("B_SyncPull_A_To_B", func(t *testing.T) {
		resp := doSyncRecords(t, userB, 0, syncCount)
		contents := collectSyncedRoundContents(resp.Records, addrA, addrB, roundTS)
		got := len(contents)
		if got < msgCount {
			t.Fatalf("B sync pull A->B not enough: want >= %d, got %d (record_count=%d)", msgCount, got, resp.RecordCount)
		}
		for i, c := range contents {
			t.Logf("B sync pulled content[%03d]=%s", i+1, c)
		}
		t.Logf("B sync pulled A->B matched=%d, record_count=%d", got, resp.RecordCount)
	})

	// ── 第五步：B 给 A 回复 50 条反馈消息 ──
	t.Run("B_ReplyTo_A", func(t *testing.T) {
		for i := 1; i <= msgCount; i++ {
			content := fmt.Sprintf("B收到%03d条消息 over %s", i, roundTS)
			gr := doPushMessage(t, userB, addrB, addrA, content)
			if gr.Result != 0 {
				t.Errorf("reply %03d push failed: result=%d message=%s", i, gr.Result, gr.Message)
			}
		}
		t.Logf("B → A: %d replies sent, timestamp=%s", msgCount, roundTS)
	})

	// ── 第六步：校验 B→A 本轮回复已落库（发送方视角）──
	t.Run("B_Query_B_To_A_Stored", func(t *testing.T) {
		// pri-chat-record 为发送方视角，这里用 B 账号查询 B->A 方向记录来验证本轮回复已落库。
		resp := doGetPriRecords(t, userB, addrA, fetchCount)
		got := countRoundMessages(resp.Records, addrB, addrA, roundTS)
		if got < msgCount {
			t.Fatalf("B->A stored records not enough: want >= %d, got %d (record_count=%d)", msgCount, got, resp.RecordCount)
		}
		t.Logf("B->A stored records matched=%d, record_count=%d", got, resp.RecordCount)
	})
}

// TestQueryPrivateChatBetweenAAndB 登录 A、B 后双向各发若干条消息，分别以双方身份查询与对方私聊记录并校验。
// 说明：/store/app/pri-chat-record 为「当前用户作为发送方」视角，需分别以 A、B 拉取 A→B 与 B→A 方向记录。
func TestQueryPrivateChatBetweenAAndB(t *testing.T) {
	addrA := doLogin(t, userA)
	addrB := doLogin(t, userB)
	t.Logf("addrA=%s, addrB=%s", addrA, addrB)

	marker := fmt.Sprintf("pri-query-%s-%d", formatTimestamp(time.Now()), time.Now().UnixNano())
	const msgEachWay = 5
	const fetchCount int64 = 50

	for i := 1; i <= msgEachWay; i++ {
		content := fmt.Sprintf("[A→B #%d] %s", i, marker)
		gr := doPushMessage(t, userA, addrA, addrB, content)
		if gr.Result != 0 {
			t.Fatalf("A push #%d failed: result=%d message=%s", i, gr.Result, gr.Message)
		}
	}
	for i := 1; i <= msgEachWay; i++ {
		content := fmt.Sprintf("[B→A #%d] %s", i, marker)
		gr := doPushMessage(t, userB, addrB, addrA, content)
		if gr.Result != 0 {
			t.Fatalf("B push #%d failed: result=%d message=%s", i, gr.Result, gr.Message)
		}
	}

	// 以 A 身份查询与 B 的私聊：应能看到 A 发往 B 的记录
	respAB := doGetPriRecords(t, userA, addrB, fetchCount)
	gotAB := countRoundMessages(respAB.Records, addrA, addrB, marker)
	if gotAB < msgEachWay {
		t.Fatalf("A 查询 A→B 记录不足: want >= %d, got %d (record_count=%d)", msgEachWay, gotAB, respAB.RecordCount)
	}
	t.Logf("A 视角 pri-chat-record(target=B): A→B 命中=%d, record_count=%d", gotAB, respAB.RecordCount)

	// 以 B 身份查询与 A 的私聊：应能看到 B 发往 A 的记录
	respBA := doGetPriRecords(t, userB, addrA, fetchCount)
	gotBA := countRoundMessages(respBA.Records, addrB, addrA, marker)
	if gotBA < msgEachWay {
		t.Fatalf("B 查询 B→A 记录不足: want >= %d, got %d (record_count=%d)", msgEachWay, gotBA, respBA.RecordCount)
	}
	t.Logf("B 视角 pri-chat-record(target=A): B→A 命中=%d, record_count=%d", gotBA, respBA.RecordCount)
}

// TestQueryPrivateChatBetweenAAndB_WithoutPush 仅登录后直连查询双方私聊历史，不调用 /record/push。
// 校验：接口成功、record_count 与返回条数一致、每条记录符合「当前用户为发送方、targetId 为对方」的语义。
func TestQueryPrivateChatBetweenAAndB_WithoutPush(t *testing.T) {
	addrA := doLogin(t, userA)
	addrB := doLogin(t, userB)
	t.Logf("addrA=%s, addrB=%s", addrA, addrB)

	const fetchCount int64 = 50

	respAB := doGetPriRecords(t, userA, addrB, fetchCount)
	if respAB.RecordCount != len(respAB.Records) {
		t.Fatalf("A 查询: record_count 与 records 长度不一致: record_count=%d, len=%d", respAB.RecordCount, len(respAB.Records))
	}
	for i, r := range respAB.Records {
		if r.FromID != addrA || r.TargetID != addrB {
			t.Fatalf("A 查询 records[%d] 与发送方视角不符: fromId=%q targetId=%q, want from=%q target=%q",
				i, r.FromID, r.TargetID, addrA, addrB)
		}
	}
	t.Logf("A 视角 pri-chat-record(target=B): 条数=%d", respAB.RecordCount)
	logPriRecordsDetail(t, "A→B 私聊（用户 A 查询，targetId=B）", respAB.Records)

	respBA := doGetPriRecords(t, userB, addrA, fetchCount)
	if respBA.RecordCount != len(respBA.Records) {
		t.Fatalf("B 查询: record_count 与 records 长度不一致: record_count=%d, len=%d", respBA.RecordCount, len(respBA.Records))
	}
	for i, r := range respBA.Records {
		if r.FromID != addrB || r.TargetID != addrA {
			t.Fatalf("B 查询 records[%d] 与发送方视角不符: fromId=%q targetId=%q, want from=%q target=%q",
				i, r.FromID, r.TargetID, addrB, addrA)
		}
	}
	t.Logf("B 视角 pri-chat-record(target=A): 条数=%d", respBA.RecordCount)
	logPriRecordsDetail(t, "B→A 私聊（用户 B 查询，targetId=A）", respBA.Records)
}

// ─── 聊天记录清理测试 ─────────────────────────────────────────────────────────

// TestCleanPrivateChat_Last7Days 测试手动删除"最近 7 天"聊天记录。
func TestCleanPrivateChat_Last7Days(t *testing.T) {
	gr := doPost(t, "/app/record/clean-private", userA, CleanPrivateChatReq{
		TargetID: addressOfUser(userB),
		DateOpt:  3, // LAST_7_DAYS
	})
	assertOK(t, gr)

	var resp CleanPrivateChatResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	t.Logf("deleted %d messages (last 7 days)", resp.DeletedCount)
}

// TestCleanPrivateChat_AllTime 测试手动删除全部聊天记录。
func TestCleanPrivateChat_AllTime(t *testing.T) {
	gr := doPost(t, "/app/record/clean-private", userA, CleanPrivateChatReq{
		TargetID: addressOfUser(userB),
		DateOpt:  8, // ALL_TIME
	})
	assertOK(t, gr)

	var resp CleanPrivateChatResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	t.Logf("deleted %d messages (all time)", resp.DeletedCount)
}

// TestCleanPrivateChat_Custom 测试手动删除"自定义时间范围"。
func TestCleanPrivateChat_Custom(t *testing.T) {
	now := time.Now().UnixMilli()
	weekAgo := now - int64(7*24*time.Hour/time.Millisecond)

	gr := doPost(t, "/app/record/clean-private", userA, CleanPrivateChatReq{
		TargetID:  addressOfUser(userB),
		DateOpt:   0, // CUSTOM
		StartTime: weekAgo,
		EndTime:   now,
	})
	assertOK(t, gr)

	var resp CleanPrivateChatResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	t.Logf("deleted %d messages (custom range)", resp.DeletedCount)
}

// TestCleanPrivateChat_InvalidCustomRange 验证自定义范围参数校验。
func TestCleanPrivateChat_InvalidCustomRange(t *testing.T) {
	gr := doPost(t, "/app/record/clean-private", userA, CleanPrivateChatReq{
		TargetID:  addressOfUser(userB),
		DateOpt:   0,
		StartTime: 0,
		EndTime:   0,
	})
	if gr.Result == 0 {
		t.Error("expected error for invalid custom range, got result=0")
	}
	t.Logf("got expected error: result=%d message=%s", gr.Result, gr.Message)
}

// TestSetAutoDeleteConfig_7Days 测试设置 7 天自动删除。
func TestSetAutoDeleteConfig_7Days(t *testing.T) {
	gr := doPost(t, "/app/record/auto-delete/set", userA, SetAutoDeleteConfigReq{
		TargetID: addressOfUser(userB),
		Option:   3, // AUTO_7_DAYS
	})
	assertOK(t, gr)

	var resp AutoDeleteConfigResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if resp.Option != 3 {
		t.Errorf("expected option=3, got %d", resp.Option)
	}
	t.Logf("auto-delete set: option=%d updatedBy=%s updatedAt=%d", resp.Option, resp.UpdatedBy, resp.UpdatedAt)
}

// TestSetAutoDeleteConfig_Never 测试关闭自动删除（设为 0）。
func TestSetAutoDeleteConfig_Never(t *testing.T) {
	gr := doPost(t, "/app/record/auto-delete/set", userA, SetAutoDeleteConfigReq{
		TargetID: addressOfUser(userB),
		Option:   0, // AUTO_NEVER
	})
	assertOK(t, gr)

	var resp AutoDeleteConfigResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if resp.Option != 0 {
		t.Errorf("expected option=0, got %d", resp.Option)
	}
	t.Logf("auto-delete disabled: option=%d", resp.Option)
}

// TestGetAutoDeleteConfig_Default 测试查询未配置时返回默认值 0（从不删除）。
func TestGetAutoDeleteConfig_Default(t *testing.T) {
	doPost(t, "/app/record/auto-delete/set", userA, SetAutoDeleteConfigReq{
		TargetID: addressOfUser(userB),
		Option:   0,
	})

	gr := doPost(t, "/app/record/auto-delete/get", userA, GetAutoDeleteConfigReq{
		TargetID: addressOfUser(userB),
	})
	assertOK(t, gr)

	var resp AutoDeleteConfigResp
	if err := json.Unmarshal(gr.Data, &resp); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if resp.Option != 0 {
		t.Errorf("expected option=0 (never), got %d", resp.Option)
	}
	t.Logf("auto-delete config: option=%d", resp.Option)
}

// TestAutoDeleteConfigRoundTrip 完整的"设置 → 读取 → 修改 → 再读取"流程。
func TestAutoDeleteConfigRoundTrip(t *testing.T) {
	options := []int8{1, 2, 3, 4, 5, 0}
	for _, opt := range options {
		t.Run(fmt.Sprintf("option=%d", opt), func(t *testing.T) {
			gr := doPost(t, "/app/record/auto-delete/set", userA, SetAutoDeleteConfigReq{
				TargetID: addressOfUser(userB),
				Option:   opt,
			})
			assertOK(t, gr)

			gr2 := doPost(t, "/app/record/auto-delete/get", userA, GetAutoDeleteConfigReq{
				TargetID: addressOfUser(userB),
			})
			assertOK(t, gr2)

			var resp AutoDeleteConfigResp
			if err := json.Unmarshal(gr2.Data, &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Option != opt {
				t.Errorf("expected option=%d after set, got %d", opt, resp.Option)
			}
		})
	}
}

// TestCleanPrivateChat_DateOptions 遍历所有预设时间范围选项。
func TestCleanPrivateChat_DateOptions(t *testing.T) {
	opts := []struct {
		opt  int32
		name string
	}{
		{1, "LAST_1_DAY"},
		{2, "LAST_3_DAYS"},
		{3, "LAST_7_DAYS"},
		{4, "LAST_30_DAYS"},
		{5, "LAST_90_DAYS"},
		{6, "LAST_180_DAYS"},
		{7, "LAST_365_DAYS"},
		{8, "ALL_TIME"},
	}
	for _, tc := range opts {
		t.Run(tc.name, func(t *testing.T) {
			gr := doPost(t, "/app/record/clean-private", userA, CleanPrivateChatReq{
				TargetID: addressOfUser(userB),
				DateOpt:  tc.opt,
			})
			assertOK(t, gr)
			var resp CleanPrivateChatResp
			if err := json.Unmarshal(gr.Data, &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			t.Logf("[%s] deleted=%d", tc.name, resp.DeletedCount)
		})
	}
}

// TestCleanPrivateChat_MissingTarget 测试缺少 targetId 时的参数校验。
func TestCleanPrivateChat_MissingTarget(t *testing.T) {
	gr := doPost(t, "/app/record/clean-private", userA, map[string]interface{}{
		"dateOpt": 3,
	})
	if gr.Result == 0 {
		t.Error("expected params error, got result=0")
	}
	t.Logf("got expected error: result=%d message=%s", gr.Result, gr.Message)
}

// TestAutoDelete_SetAndGet_UserB 验证 B 用户视角也能独立设置/查询配置。
func TestAutoDelete_SetAndGet_UserB(t *testing.T) {
	gr := doPost(t, "/app/record/auto-delete/set", userB, SetAutoDeleteConfigReq{
		TargetID: addressOfUser(userA),
		Option:   2, // AUTO_3_DAYS
	})
	assertOK(t, gr)

	gr2 := doPost(t, "/app/record/auto-delete/get", userB, GetAutoDeleteConfigReq{
		TargetID: addressOfUser(userA),
	})
	assertOK(t, gr2)

	var resp AutoDeleteConfigResp
	if err := json.Unmarshal(gr2.Data, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Option != 2 {
		t.Errorf("expected option=2, got %d", resp.Option)
	}
	t.Logf("user B auto-delete config: option=%d updatedBy=%s", resp.Option, resp.UpdatedBy)
}

// ─── 认证异常场景测试 ──────────────────────────────────────────────────────────

// TestCleanPrivateChat_NoSignature 不带 FZM-SIGNATURE 时应返回认证错误。
func TestCleanPrivateChat_NoSignature(t *testing.T) {
	raw, _ := json.Marshal(CleanPrivateChatReq{
		TargetID: addressOfUser(userB),
		DateOpt:  3,
	})
	req, err := http.NewRequest(http.MethodPost, envGatewayBase()+"/app/record/clean-private", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	t.Logf("← %d %s", resp.StatusCode, b)

	var gr GeneralResponse
	if err := json.Unmarshal(b, &gr); err != nil {
		t.Logf("response is not JSON (expected for auth failure): %s", b)
		return
	}
	if gr.Result == 0 {
		t.Error("expected auth error without FZM-SIGNATURE, got result=0")
	}
	t.Logf("got expected auth error: result=%d message=%s", gr.Result, gr.Message)
}

// TestCleanPrivateChat_InvalidSignature 使用无效签名时应返回 SignatureInvalid。
func TestCleanPrivateChat_InvalidSignature(t *testing.T) {
	raw, _ := json.Marshal(CleanPrivateChatReq{
		TargetID: addressOfUser(userB),
		DateOpt:  3,
	})
	req, err := http.NewRequest(http.MethodPost, envGatewayBase()+"/app/record/clean-private", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("FZM-SIGNATURE", "this-is-an-invalid-signature-value")
	req.Header.Set("FZM-UUID", uuidA)
	req.Header.Set("FZM-DEVICE", fzmDevice)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http do: %v", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	t.Logf("← %d %s", resp.StatusCode, b)

	var gr GeneralResponse
	if err := json.Unmarshal(b, &gr); err != nil {
		t.Logf("response is not JSON (expected for auth failure): %s", b)
		return
	}
	if gr.Result == 0 {
		t.Error("expected auth error with invalid signature, got result=0")
	}
	t.Logf("got expected auth error: result=%d message=%s", gr.Result, gr.Message)
}
