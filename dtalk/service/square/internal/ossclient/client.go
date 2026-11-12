package ossclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/pkg/address"
	"github.com/web3chat/txchat/dtalk/service/square/pkg/auth"
)

const (
	MaxBytes      = 2 << 20
	sniffBytes    = 512
	uploadTimeout = 45 * time.Second
)

var (
	ErrNotConfigured = errors.New("oss not configured")
	ErrNoSigner      = errors.New("oss signer not configured")
	ErrTooLarge      = errors.New("image too large")
	ErrNotImage      = errors.New("not an image")
	ErrEmpty         = errors.New("empty file")
)

type Client struct {
	endpoint      string
	deleteURL     string
	appID         string
	priv          []byte
	pub           []byte
	addr          string
	http          *http.Client
	log           *slog.Logger
	deleteEnabled bool
}

func New(cfg config.OSSConfig) *Client {
	appID := strings.TrimSpace(cfg.AppID)
	if appID == "" {
		appID = "dtalk"
	}
	c := &Client{
		endpoint:      cfg.UploadURL(),
		deleteURL:     cfg.DeleteURL(),
		appID:         appID,
		http:          &http.Client{Timeout: uploadTimeout},
		log:           slog.Default(),
		deleteEnabled: cfg.DeleteEnabled,
	}
	priv, pub, err := parseOSSPrivateKey(cfg.PrivKey)
	if err != nil {
		slog.Error("oss signer invalid", "err", err)
		return c
	}
	c.priv = priv
	c.pub = pub
	c.addr = address.PublicKeyToAddress(address.NormalVer, pub)
	slog.Info("oss signer ready", "addr", c.addr, "appId", c.appID)
	return c
}

func (c *Client) Enabled() bool {
	return c != nil && c.endpoint != "" && len(c.priv) == 32 && len(c.pub) > 0
}

func (c *Client) Endpoint() string {
	if c == nil {
		return ""
	}
	return c.endpoint
}

func (c *Client) SignerAddr() string {
	if c == nil {
		return ""
	}
	return c.addr
}

func (c *Client) DeleteEnabled() bool {
	return c != nil && c.deleteEnabled
}

func (c *Client) Delete(ctx context.Context, fileURL string) error {
	if c == nil || !c.deleteEnabled {
		return nil
	}
	if !c.Enabled() || strings.TrimSpace(c.deleteURL) == "" {
		return ErrNotConfigured
	}
	fileURL = strings.TrimSpace(fileURL)
	if fileURL == "" {
		return nil
	}
	sig, err := c.signOSS()
	if err != nil {
		return fmt.Errorf("sign oss delete: %w", err)
	}
	payload, err := json.Marshal(map[string]string{"url": fileURL})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.deleteURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("FZM-SIGNATURE", sig)
	resp, err := c.http.Do(req)
	if err != nil {
		c.log.Error("oss delete request failed", "endpoint", c.deleteURL, "url", fileURL, "err", err)
		return fmt.Errorf("oss delete request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		c.log.Error("oss delete rejected", "endpoint", c.deleteURL, "url", fileURL, "status", resp.StatusCode, "body", truncate(string(raw), 400))
		return fmt.Errorf("oss delete status %d", resp.StatusCode)
	}
	if parseErr := parseOSSDeleteAck(raw); parseErr != nil {
		c.log.Error("oss delete ack failed", "endpoint", c.deleteURL, "url", fileURL, "body", truncate(string(raw), 400), "err", parseErr)
		return parseErr
	}
	c.log.Info("oss delete ok", "endpoint", c.deleteURL, "url", fileURL)
	return nil
}

func (c *Client) Upload(ctx context.Context, kind, filename string, size int64, body io.Reader) (string, error) {
	if !c.Enabled() {
		return "", ErrNotConfigured
	}
	if size <= 0 {
		return "", ErrEmpty
	}
	if size > MaxBytes {
		return "", ErrTooLarge
	}
	head := make([]byte, sniffBytes)
	n, err := io.ReadFull(body, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read image head: %w", err)
	}
	head = head[:n]
	if len(head) == 0 {
		return "", ErrEmpty
	}
	ext, ok := imageExt(filename, head)
	if !ok {
		return "", ErrNotImage
	}
	key := objectKey(kind, ext)
	sig, err := c.signOSS()
	if err != nil {
		return "", fmt.Errorf("sign oss: %w", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("appId", c.appID); err != nil {
		return "", err
	}
	if err := mw.WriteField("key", key); err != nil {
		return "", err
	}
	part, err := mw.CreateFormFile("file", safeFilename(filename, ext))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(head); err != nil {
		return "", err
	}
	if _, err := io.Copy(part, io.LimitReader(body, MaxBytes+1-int64(len(head)))); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("FZM-SIGNATURE", sig)

	resp, err := c.http.Do(req)
	if err != nil {
		c.log.Error("oss upload request failed", "endpoint", c.endpoint, "signer", c.addr, "key", key, "size", size, "err", err)
		return "", fmt.Errorf("oss request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	url, parseErr := parseOSSAck(raw)
	if resp.StatusCode != http.StatusOK || parseErr != nil || url == "" {
		c.log.Error("oss upload rejected",
			"endpoint", c.endpoint,
			"signer", c.addr,
			"key", key,
			"size", size,
			"status", resp.StatusCode,
			"body", truncate(string(raw), 400),
			"err", parseErr,
		)
		if parseErr != nil {
			return "", parseErr
		}
		return "", fmt.Errorf("oss status %d", resp.StatusCode)
	}
	c.log.Info("oss upload ok", "endpoint", c.endpoint, "signer", c.addr, "key", key, "size", size, "url", url)
	return url, nil
}

func parseOSSPrivateKey(hexKey string) (priv, pub []byte, err error) {
	hexKey = strings.TrimPrefix(strings.TrimSpace(hexKey), "0x")
	raw, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, nil, fmt.Errorf("SQUARE_OSS_PRIVKEY hex: %w", err)
	}
	if len(raw) != 32 {
		return nil, nil, fmt.Errorf("SQUARE_OSS_PRIVKEY must be 32 bytes, got %d", len(raw))
	}
	key, err := ethcrypto.ToECDSA(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("SQUARE_OSS_PRIVKEY ecdsa: %w", err)
	}
	return ethcrypto.FromECDSA(key), ethcrypto.CompressPubkey(&key.PublicKey), nil
}

func (c *Client) signOSS() (string, error) {
	if c == nil || len(c.priv) != 32 || len(c.pub) == 0 {
		return "", ErrNoSigner
	}
	return auth.NewDefaultApiAuthenticator().Request(c.appID, c.pub, c.priv), nil
}

func objectKey(kind, ext string) string {
	now := time.Now()
	folder := "chatList/picture/"
	if strings.EqualFold(strings.TrimSpace(kind), "avatar") {
		folder = "chatList/avatar/"
	}
	var rnd [4]byte
	_, _ = rand.Read(rnd[:])
	return fmt.Sprintf("%s%s/%s_square_%s.%s",
		folder,
		now.Format("20060102"),
		now.Format("150405")+fmt.Sprintf("%03d", now.Nanosecond()/1e6),
		hex.EncodeToString(rnd[:]),
		ext,
	)
}

func imageExt(filename string, head []byte) (string, bool) {
	ct := http.DetectContentType(head)
	switch ct {
	case "image/jpeg":
		return "jpg", true
	case "image/png":
		return "png", true
	case "image/gif":
		return "gif", true
	case "image/webp":
		return "webp", true
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(filename), "."))
	switch ext {
	case "jpg", "jpeg":
		return "jpg", true
	case "png", "gif", "webp":
		return ext, true
	}
	return "", false
}

func safeFilename(name, ext string) string {
	base := path.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == "/" {
		return "image." + ext
	}
	if !strings.Contains(base, ".") {
		return base + "." + ext
	}
	return base
}

type ossAck struct {
	Result  json.RawMessage `json:"result"`
	Message json.RawMessage `json:"message"`
	Data    struct {
		URL string `json:"url"`
		URI string `json:"uri"`
	} `json:"data"`
}

func parseOSSAck(raw []byte) (string, error) {
	var ack ossAck
	if err := json.Unmarshal(raw, &ack); err != nil {
		return "", fmt.Errorf("oss response json: %w", err)
	}
	if !ossResultOK(ack.Result) {
		msg := strings.TrimSpace(string(ack.Message))
		msg = strings.Trim(msg, `"`)
		if msg == "" {
			msg = "oss upload failed"
		}
		return "", fmt.Errorf("oss result %s: %s", strings.TrimSpace(string(ack.Result)), msg)
	}
	url := strings.TrimSpace(ack.Data.URL)
	if url == "" {
		url = strings.TrimSpace(ack.Data.URI)
	}
	if url == "" {
		return "", errors.New("oss empty url")
	}
	return url, nil
}

func parseOSSDeleteAck(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var ack ossAck
	if err := json.Unmarshal(raw, &ack); err != nil {
		return fmt.Errorf("oss delete response json: %w", err)
	}
	if !ossResultOK(ack.Result) {
		msg := strings.TrimSpace(string(ack.Message))
		msg = strings.Trim(msg, `"`)
		if msg == "" {
			msg = "oss delete failed"
		}
		return fmt.Errorf("oss result %s: %s", strings.TrimSpace(string(ack.Result)), msg)
	}
	return nil
}

func ossResultOK(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "0" || s == `"0"`
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
