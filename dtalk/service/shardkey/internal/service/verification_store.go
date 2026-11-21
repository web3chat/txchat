package service

import (
	"encoding/json"
	"sync"
	"time"

	goredis "github.com/gomodule/redigo/redis"

	dtredis "github.com/web3chat/txchat/dtalk/pkg/redis"
)

// VerificationCode 验证码记录，落 Redis，多实例 / 重启不丢。
type VerificationCode struct {
	Code      string    `json:"code"`
	Phone     string    `json:"phone"`
	Purpose   string    `json:"purpose"`
	CreatedAt time.Time `json:"created_at"`
}

// VerificationStore 验证码存储抽象；线上用 Redis，单测用内存实现。
type VerificationStore interface {
	TrySendLock(phone string, ttl time.Duration) (bool, error)
	ReleaseSendLock(phone string) error
	Save(phone string, rec *VerificationCode, ttl time.Duration) error
	Get(phone string) (*VerificationCode, error)
	Delete(phone string) error
	IncrAttempts(phone string, ttl time.Duration) (int64, error)
	DeleteAttempts(phone string) error
}

// RedisVerificationStore 基于 dtalk/pkg/redis 的实现。
type RedisVerificationStore struct {
	pool *dtredis.Pool
}

func NewRedisVerificationStore(pool *dtredis.Pool) *RedisVerificationStore {
	return &RedisVerificationStore{pool: pool}
}

func codeKey(phone string) string    { return "shardkey:code:" + phone }
func sendKey(phone string) string    { return "shardkey:send:" + phone }
func attemptKey(phone string) string { return "shardkey:wrong:" + phone }

func (s *RedisVerificationStore) TrySendLock(phone string, ttl time.Duration) (bool, error) {
	reply, err := s.pool.Do("SET", sendKey(phone), "1", "EX", int(ttl.Seconds()), "NX")
	if err != nil {
		return false, err
	}
	return reply != nil, nil
}

func (s *RedisVerificationStore) ReleaseSendLock(phone string) error {
	_, err := s.pool.Del(sendKey(phone))
	return err
}

func (s *RedisVerificationStore) Save(phone string, rec *VerificationCode, ttl time.Duration) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = s.pool.Do("SET", codeKey(phone), data, "EX", int(ttl.Seconds()))
	return err
}

func (s *RedisVerificationStore) Get(phone string) (*VerificationCode, error) {
	data, err := goredis.Bytes(s.pool.Do("GET", codeKey(phone)))
	if err == goredis.ErrNil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec := &VerificationCode{}
	if err := json.Unmarshal(data, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func (s *RedisVerificationStore) Delete(phone string) error {
	_, err := s.pool.Del(codeKey(phone))
	return err
}

func (s *RedisVerificationStore) IncrAttempts(phone string, ttl time.Duration) (int64, error) {
	n, err := goredis.Int64(s.pool.Do("INCR", attemptKey(phone)))
	if err != nil {
		return 0, err
	}
	if n == 1 {
		if _, err := s.pool.Do("EXPIRE", attemptKey(phone), int(ttl.Seconds())); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (s *RedisVerificationStore) DeleteAttempts(phone string) error {
	_, err := s.pool.Del(attemptKey(phone))
	return err
}

// MemoryVerificationStore 供单测使用，不依赖 Redis。
type MemoryVerificationStore struct {
	mu      sync.Mutex
	codes   map[string]*VerificationCode
	locks   map[string]time.Time
	attempt map[string]int64
}

func NewMemoryVerificationStore() *MemoryVerificationStore {
	return &MemoryVerificationStore{
		codes:   make(map[string]*VerificationCode),
		locks:   make(map[string]time.Time),
		attempt: make(map[string]int64),
	}
}

func (m *MemoryVerificationStore) TrySendLock(phone string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if until, ok := m.locks[phone]; ok && time.Now().Before(until) {
		return false, nil
	}
	m.locks[phone] = time.Now().Add(ttl)
	return true, nil
}

func (m *MemoryVerificationStore) ReleaseSendLock(phone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.locks, phone)
	return nil
}

func (m *MemoryVerificationStore) Save(phone string, rec *VerificationCode, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyRec := *rec
	m.codes[phone] = &copyRec
	return nil
}

func (m *MemoryVerificationStore) Get(phone string) (*VerificationCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.codes[phone]
	if !ok {
		return nil, nil
	}
	copyRec := *rec
	return &copyRec, nil
}

func (m *MemoryVerificationStore) Delete(phone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.codes, phone)
	return nil
}

func (m *MemoryVerificationStore) IncrAttempts(phone string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempt[phone]++
	return m.attempt[phone], nil
}

func (m *MemoryVerificationStore) DeleteAttempts(phone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.attempt, phone)
	return nil
}
