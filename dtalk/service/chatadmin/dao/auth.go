package dao

import (
	"context"
	"strings"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
)

// AdminWhitelist 管理员白名单实体
type AdminWhitelist struct {
	ID        int64
	Address   string
	Remark    string
	Status    int32
	Level     int32 // 管理员级别: 0=普通管理员, 1=超级管理员
	CreatedBy string
	CreatedAt int64
	UpdatedAt int64
}

// AuthNonce 一次性登录随机串
type AuthNonce struct {
	Address  string
	Nonce    string
	ExpireAt int64
}

const (
	_CreateAdminWhitelistTable = `CREATE TABLE IF NOT EXISTS groupmanage_admin_whitelist (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        address VARCHAR(64) NOT NULL UNIQUE,
        remark VARCHAR(255) DEFAULT '',
        status TINYINT NOT NULL DEFAULT 1,
        level TINYINT NOT NULL DEFAULT 0,
        created_by VARCHAR(64) DEFAULT '',
        created_at BIGINT NOT NULL,
        updated_at BIGINT NOT NULL
    )`

	_CreateAuthNonceTable = `CREATE TABLE IF NOT EXISTS groupmanage_auth_nonce (
        address VARCHAR(64) PRIMARY KEY,
        nonce VARCHAR(128) NOT NULL,
        expire_at BIGINT NOT NULL
    )`

	_UpsertNonce       = `REPLACE INTO groupmanage_auth_nonce(address, nonce, expire_at) VALUES(?,?,?)`
	_GetNonceByAddr    = `SELECT * FROM groupmanage_auth_nonce WHERE address=?`
	_DeleteNonceByAddr = `DELETE FROM groupmanage_auth_nonce WHERE address=?`

	_GetWhitelistByAddr = `SELECT * FROM groupmanage_admin_whitelist WHERE address=? AND status=1`
	_GetAdminLevel      = `SELECT level FROM groupmanage_admin_whitelist WHERE address=? AND status=1`
)

type AuthDAO interface {
	EnsureTables(ctx context.Context) error
	UpsertNonce(ctx context.Context, nonce *AuthNonce) error
	GetNonce(ctx context.Context, address string) (*AuthNonce, error)
	DeleteNonce(ctx context.Context, address string) error
	IsAddressAllowed(ctx context.Context, address string) (bool, error)
	GetAdminLevel(ctx context.Context, address string) (int32, error) // 获取管理员级别，0=普通管理员，1=超级管理员
}

type authDAO struct {
	mysql *mysql.MysqlConn
}

func NewAuthDAO(mysql *mysql.MysqlConn) AuthDAO {
	return &authDAO{mysql: mysql}
}

func (d *authDAO) EnsureTables(ctx context.Context) error {
	if _, _, err := d.mysql.Exec(_CreateAdminWhitelistTable); err != nil {
		return err
	}
	if _, _, err := d.mysql.Exec(_CreateAuthNonceTable); err != nil {
		return err
	}
	return nil
}

func (d *authDAO) UpsertNonce(ctx context.Context, nonce *AuthNonce) error {
	addr := strings.ToLower(nonce.Address)
	_, _, err := d.mysql.Exec(_UpsertNonce, addr, nonce.Nonce, nonce.ExpireAt)
	return err
}

func (d *authDAO) GetNonce(ctx context.Context, address string) (*AuthNonce, error) {
	addr := strings.ToLower(address)
	records, err := d.mysql.Query(_GetNonceByAddr, addr)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	r := records[0]
	n := &AuthNonce{Address: addr}
	if v, ok := r["nonce"]; ok {
		n.Nonce = v
	}
	if v, ok := r["expire_at"]; ok {
		// parse to int64 milliseconds
		// ignore error and leave zero on failure
		// local small parser
		var x int64
		for i := 0; i < len(v); i++ {
			c := v[i]
			if c < '0' || c > '9' {
				x = 0
				break
			}
			x = x*10 + int64(c-'0')
		}
		n.ExpireAt = x
	}
	return n, nil
}

func (d *authDAO) DeleteNonce(ctx context.Context, address string) error {
	addr := strings.ToLower(address)
	_, _, err := d.mysql.Exec(_DeleteNonceByAddr, addr)
	return err
}

func (d *authDAO) IsAddressAllowed(ctx context.Context, address string) (bool, error) {
	addr := strings.ToLower(address)
	records, err := d.mysql.Query(_GetWhitelistByAddr, addr)
	if err != nil {
		return false, err
	}
	return len(records) > 0, nil
}

// GetAdminLevel 获取管理员级别
// 返回: 0=普通管理员, 1=超级管理员, -1=未找到或已禁用
func (d *authDAO) GetAdminLevel(ctx context.Context, address string) (int32, error) {
	addr := strings.ToLower(address)
	records, err := d.mysql.Query(_GetAdminLevel, addr)
	if err != nil {
		return -1, err
	}
	if len(records) == 0 {
		return -1, nil // 未找到或已禁用
	}
	r := records[0]
	if v, ok := r["level"]; ok {
		// 解析level字段
		var level int32
		for i := 0; i < len(v); i++ {
			c := v[i]
			if c < '0' || c > '9' {
				return 0, nil // 解析失败，默认返回普通管理员
			}
			level = level*10 + int32(c-'0')
		}
		return level, nil
	}
	return 0, nil // 默认返回普通管理员
}
