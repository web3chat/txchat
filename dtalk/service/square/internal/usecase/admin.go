package usecase

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const (
	captchaTTL           = 2 * time.Minute
	loginIPLockAfter     = 3
	loginIPLockFor       = time.Hour
	loginIPPerMin        = 20
	loginAlertWindow     = time.Hour
	loginAlertMinIPs     = 2
	loginAlertMinFails   = 2
	loginAlertIPLimit    = 8
	loginShareMinIPs     = 2
	loginMultiMinSession = 2
	bcryptCost           = 12
	minPasswordLen       = 12
)

type Claims struct {
	AdminID     int64    `json:"adminId"`
	Username    string   `json:"username"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

func (c *Claims) Can(code string) bool {
	if c == nil {
		return false
	}
	return entity.HasPerm(c.Permissions, code)
}

func (s *Service) BootstrapAdmin(ctx context.Context) error {
	accounts := s.bootstrapAccounts()
	if len(accounts) == 0 {
		n, err := s.store.CountAdmins(ctx)
		if err != nil {
			s.log.Error("count admins failed", "err", err)
			return err
		}
		if n == 0 {
			s.log.Warn("no bootstrap admin; set SQUARE_BOOTSTRAP_ADMIN_USER/PASSWORD")
		}
	} else {
		for _, account := range accounts {
			if err := s.ensureBootstrapAdmin(ctx, account); err != nil {
				return err
			}
		}
	}
	return s.backfillAdminPerms(ctx)
}

func (s *Service) bootstrapAccounts() []config.BootstrapAccount {
	var list []config.BootstrapAccount
	if user := strings.TrimSpace(s.cfg.Bootstrap.AdminUser); user != "" {
		list = append(list, config.BootstrapAccount{
			User: user,
			Pass: s.cfg.Bootstrap.AdminPass,
			Role: entity.RoleSuper,
		})
	}
	list = append(list, s.cfg.Bootstrap.Accounts...)
	seen := make(map[string]struct{}, len(list))
	out := make([]config.BootstrapAccount, 0, len(list))
	for _, account := range list {
		user := strings.TrimSpace(account.User)
		if user == "" {
			continue
		}
		if _, ok := seen[user]; ok {
			continue
		}
		seen[user] = struct{}{}
		account.User = user
		out = append(out, account)
	}
	return out
}

func (s *Service) ensureBootstrapAdmin(ctx context.Context, account config.BootstrapAccount) error {
	user := account.User
	role := strings.TrimSpace(account.Role)
	if len(account.Pass) < minPasswordLen {
		s.log.Warn("skip bootstrap admin", "user", user, "reason", "password shorter than 12")
		return nil
	}
	if role != entity.RoleSuper && role != entity.RoleReviewer {
		s.log.Warn("skip bootstrap admin", "user", user, "role", role, "reason", "role must be super or reviewer")
		return nil
	}
	existing, err := s.store.GetAdminByUsername(ctx, user)
	if err != nil {
		s.log.Error("bootstrap admin lookup failed", "user", user, "err", err)
		return err
	}
	if existing != nil {
		if resetThisBootstrap(s.cfg.Bootstrap.ResetAdminPass, s.cfg.Bootstrap.AdminUser, user) {
			return s.overwriteAdminPassword(ctx, existing, account.Pass)
		}
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(account.Pass), bcryptCost)
	if err != nil {
		s.log.Error("bootstrap admin hash failed", "user", user, "err", err)
		return err
	}
	root := strings.TrimSpace(s.cfg.Bootstrap.AdminUser)
	a := &entity.Admin{
		Username:     user,
		PasswordHash: string(hash),
		Role:         role,
		Status:       1,
		Permissions:  entity.DefaultPerms(role, root != "" && user == root),
		PermReady:    true,
	}
	if err := s.store.InsertAdmin(ctx, a); err != nil {
		s.log.Error("bootstrap admin failed", "user", user, "role", role, "err", err)
		return err
	}
	s.log.Info("bootstrap admin created", "user", user, "role", role, "permissions", a.Permissions)
	return nil
}

// resetThisBootstrap 只允许用环境变量覆盖根管理员，不改 SQUARE_BOOTSTRAP_ADMINS 里的其他账号。
func resetThisBootstrap(reset bool, root, user string) bool {
	root = strings.TrimSpace(root)
	return reset && root != "" && root == user
}

func (s *Service) overwriteAdminPassword(ctx context.Context, admin *entity.Admin, pass string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcryptCost)
	if err != nil {
		s.log.Error("reset admin password hash failed", "id", admin.ID, "user", admin.Username, "err", err)
		return err
	}
	if err = s.store.UpdateAdminPassword(ctx, admin.ID, string(hash)); err != nil {
		s.log.Error("reset admin password failed", "id", admin.ID, "user", admin.Username, "err", err)
		return err
	}
	if err = s.store.ClearGuard(ctx, "u:"+admin.Username); err != nil {
		s.log.Error("clear admin login guard after reset failed", "id", admin.ID, "user", admin.Username, "err", err)
	}
	s.log.Warn("admin password reset from env", "id", admin.ID, "user", admin.Username, "hint", "set SQUARE_ADMIN_PASSWORD_RESET=false after login")
	return nil
}

func validateAdminPassword(pass string) error {
	if len(pass) < minPasswordLen {
		return xerror.New(xerror.ParamInvalid, "密码至少 12 位")
	}
	return nil
}

func (s *Service) ChangeAdminPassword(ctx context.Context, adminID int64, current, next string) error {
	if err := validateAdminPassword(next); err != nil {
		s.log.Warn("reject admin password change", "adminId", adminID, "reason", "too short", "len", len(next))
		return err
	}
	if current == next {
		s.log.Warn("reject admin password change", "adminId", adminID, "reason", "same as current")
		return xerror.New(xerror.ParamInvalid, "新密码不能与当前密码相同")
	}
	admin, err := s.store.GetAdminByID(ctx, adminID)
	if err != nil {
		s.log.Error("change admin password lookup failed", "adminId", adminID, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if admin == nil || admin.Status != 1 {
		s.log.Warn("change admin password missing", "adminId", adminID)
		return xerror.New(xerror.AdminUnauthorized, "未登录")
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(current)) != nil {
		s.log.Warn("change admin password rejected", "adminId", adminID, "user", admin.Username, "reason", "current mismatch")
		return xerror.New(xerror.AdminBadCred, "当前密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcryptCost)
	if err != nil {
		s.log.Error("change admin password hash failed", "adminId", adminID, "user", admin.Username, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err = s.store.UpdateAdminPassword(ctx, admin.ID, string(hash)); err != nil {
		s.log.Error("change admin password failed", "adminId", adminID, "user", admin.Username, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err = s.store.ClearGuard(ctx, "u:"+admin.Username); err != nil {
		s.log.Error("clear admin login guard after change failed", "adminId", adminID, "user", admin.Username, "err", err)
	}
	s.log.Info("admin password changed", "adminId", adminID, "user", admin.Username)
	return nil
}

func (s *Service) backfillAdminPerms(ctx context.Context) error {
	list, err := s.store.ListAdmins(ctx)
	if err != nil {
		s.log.Error("list admins for perm backfill failed", "err", err)
		return err
	}
	root := strings.TrimSpace(s.cfg.Bootstrap.AdminUser)
	for _, a := range list {
		if a.PermReady {
			continue
		}
		perms := entity.DefaultPerms(a.Role, root != "" && a.Username == root)
		raw := entity.FormatPerms(perms)
		if err := s.store.UpdateAdminPermissions(ctx, a.ID, raw); err != nil {
			s.log.Error("backfill admin permissions failed", "id", a.ID, "user", a.Username, "err", err)
			return err
		}
		s.log.Info("backfill admin permissions", "id", a.ID, "user", a.Username, "role", a.Role, "permissions", raw)
	}
	return nil
}

func (s *Service) AuthorizeAdmin(ctx context.Context, token string) (*Claims, error) {
	claims, err := s.ParseJWT(token)
	if err != nil {
		return nil, err
	}
	admin, err := s.store.GetAdminByID(ctx, claims.AdminID)
	if err != nil {
		s.log.Error("authorize admin lookup failed", "adminId", claims.AdminID, "user", claims.Username, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if admin == nil {
		s.log.Warn("authorize admin missing", "adminId", claims.AdminID, "user", claims.Username)
		return nil, xerror.New(xerror.AdminUnauthorized, "未登录")
	}
	if admin.Status != 1 {
		s.log.Warn("authorize admin disabled", "adminId", admin.ID, "user", admin.Username, "status", admin.Status)
		return nil, xerror.New(xerror.AdminUnauthorized, "账号已停用")
	}
	claims.Username = admin.Username
	claims.Role = admin.Role
	claims.Permissions = admin.EffectivePerms()
	if claims.ID == "" {
		return claims, nil
	}
	alive, err := s.store.TouchAdminSession(ctx, claims.ID)
	if err != nil {
		s.log.Error("touch admin session failed", "adminId", admin.ID, "user", admin.Username, "session", claims.ID, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !alive {
		s.log.Warn("reject inactive admin session", "adminId", admin.ID, "user", admin.Username, "session", claims.ID)
		return nil, xerror.New(xerror.AdminUnauthorized, "登录已失效")
	}
	return claims, nil
}

func (s *Service) NewCaptcha(ctx context.Context) (id string, pngB64 string, err error) {
	code := randomCode(5)
	id = randomID()
	sum := sha256Hex(strings.ToLower(code))
	if err := s.store.InsertCaptcha(ctx, id, sum, time.Now().Add(captchaTTL)); err != nil {
		s.log.Error("insert captcha failed", "err", err)
		return "", "", xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	img := renderCaptcha(code)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", "", err
	}
	return id, base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (s *Service) AdminLogin(ctx context.Context, username, password, captchaID, captcha, ip string) (string, *entity.Admin, error) {
	username = strings.TrimSpace(username)
	_ = s.store.InsertAudit(ctx, &entity.Audit{Username: username, Action: "login_attempt", ClientIP: ip})
	since := time.Now().Add(-time.Minute)
	if n, err := s.store.CountLoginHits(ctx, ip, since); err == nil && n > loginIPPerMin {
		return "", nil, xerror.New(xerror.RateLimited, "操作过于频繁")
	}
	block, err := s.loginBlock(ctx, ip)
	if err != nil {
		return "", nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if block != "" {
		_ = s.store.InsertAudit(ctx, &entity.Audit{Username: username, Action: "login_locked", ClientIP: ip, Extra: block})
		s.log.Warn("reject silenced admin login ip", "ip", ip, "user", username)
		return "", nil, xerror.New(xerror.AdminLocked, "请一小时后再试")
	}
	if err := s.verifyCaptcha(ctx, captchaID, captcha); err != nil {
		_ = s.store.InsertAudit(ctx, &entity.Audit{Username: username, Action: "login_fail", ClientIP: ip, Extra: "captcha"})
		return "", nil, err
	}
	admin, err := s.store.GetAdminByUsername(ctx, username)
	if err != nil {
		return "", nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if admin == nil || admin.Status != 1 {
		s.failLogin(ctx, ip)
		_ = s.store.InsertAudit(ctx, &entity.Audit{Username: username, Action: "login_fail", ClientIP: ip, Extra: "password"})
		return "", nil, xerror.New(xerror.AdminBadCred, "用户名或密码错误")
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		s.failLogin(ctx, ip)
		_ = s.store.InsertAudit(ctx, &entity.Audit{AdminID: admin.ID, Username: username, Action: "login_fail", ClientIP: ip, Extra: "password"})
		return "", nil, xerror.New(xerror.AdminBadCred, "用户名或密码错误")
	}
	_ = s.store.ClearGuard(ctx, "u:"+username)
	_ = s.store.ClearGuard(ctx, "ip:"+ip)
	sessionID := randomID()
	exp := time.Now().Add(s.cfg.JWT.Expire)
	if err = s.store.InsertAdminSession(ctx, repository.AdminSession{
		ID: sessionID, AdminID: admin.ID, Username: admin.Username, ClientIP: ip, ExpiresAt: exp,
	}); err != nil {
		s.log.Error("insert admin session failed", "adminId", admin.ID, "user", username, "ip", ip, "err", err)
		return "", nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	token, err := s.signJWT(admin, sessionID, exp)
	if err != nil {
		if revErr := s.store.RevokeAdminSession(ctx, sessionID); revErr != nil {
			s.log.Error("revoke admin session after sign failed", "adminId", admin.ID, "user", username, "session", sessionID, "err", revErr)
		}
		return "", nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	_ = s.store.InsertAudit(ctx, &entity.Audit{AdminID: admin.ID, Username: username, Action: "login_ok", ClientIP: ip})
	return token, admin, nil
}

func (s *Service) AdminLogout(ctx context.Context, sessionID, username string) error {
	if sessionID == "" {
		return nil
	}
	if err := s.store.RevokeAdminSession(ctx, sessionID); err != nil {
		s.log.Error("revoke admin session failed", "user", username, "session", sessionID, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("admin logout", "user", username, "session", sessionID)
	return nil
}

func (s *Service) ParseJWT(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected jwt method")
		}
		return []byte(s.cfg.JWT.Secret), nil
	})
	if err != nil {
		return nil, xerror.New(xerror.AdminUnauthorized, "未登录")
	}
	return claims, nil
}

func (s *Service) Audit(ctx context.Context, a *entity.Audit) {
	if err := s.store.InsertAudit(ctx, a); err != nil {
		s.log.Error("insert audit failed", "action", a.Action, "err", err)
	}
}

func (s *Service) ListAudit(ctx context.Context, f repository.AuditFilter, page entity.PageQuery) ([]entity.Audit, int64, error) {
	list, total, err := s.store.ListAudit(ctx, f, page)
	if err != nil {
		s.log.Error("list audit failed", "actions", f.Actions, "targetTypes", f.TargetTypes, "targetId", f.TargetID, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}

func (s *Service) signJWT(a *entity.Admin, sessionID string, exp time.Time) (string, error) {
	now := time.Now()
	claims := Claims{
		AdminID:  a.ID,
		Username: a.Username,
		Role:     a.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWT.Secret))
}

func (s *Service) verifyCaptcha(ctx context.Context, id, code string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(code) == "" {
		return xerror.New(xerror.AdminCaptcha, "验证码错误")
	}
	hash, exp, err := s.store.TakeCaptcha(ctx, id)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if hash == "" || time.Now().After(exp) || sha256Hex(strings.ToLower(code)) != hash {
		return xerror.New(xerror.AdminCaptcha, "验证码错误")
	}
	return nil
}

type LoginAlert struct {
	Alert      bool
	Window     time.Duration
	MinFails   int
	IPCount    int
	FailCount  int64
	IPs        []repository.LoginFailIP
	ShareAlert bool
	Shares     []SharedLogin
	MultiAlert bool
	Accounts   []AccountLogins
}

type AccountSession struct {
	ID       string
	IP       string
	LastSeen time.Time
	Current  bool
}

type AccountLogins struct {
	Username     string
	SessionCount int
	Sessions     []AccountSession
}

type SharedLoginIP struct {
	IP       string
	LastSeen time.Time
	Current  bool
}

type SharedLogin struct {
	Username string
	IPCount  int
	IPs      []SharedLoginIP
}

func shouldWarnLoginFails(ipCount int) bool {
	return ipCount >= loginAlertMinIPs
}

func (s *Service) LoginAlert(ctx context.Context, viewerIP, viewerSession string) (*LoginAlert, error) {
	since := time.Now().Add(-loginAlertWindow)
	now := time.Now()
	rows, err := s.store.ListLoginFailIPs(ctx, since, loginAlertMinFails, loginAlertIPLimit)
	if err != nil {
		s.log.Error("list login fail ips failed", "since", since.Format(time.RFC3339), "minFails", loginAlertMinFails, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	shared, err := s.store.ListSharedLoginIPs(ctx, now)
	if err != nil {
		s.log.Error("list shared login ips failed", "viewer", viewerIP, "session", viewerSession, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	multiRows, err := s.store.ListMultiLoginSessions(ctx, now)
	if err != nil {
		s.log.Error("list multi login sessions failed", "viewer", viewerIP, "session", viewerSession, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	shares := groupSharedLogins(shared, viewerIP, loginAlertIPLimit)
	accounts := groupMultiLogins(multiRows, viewerSession, loginAlertIPLimit)
	var total int64
	for _, row := range rows {
		total += row.Fails
	}
	return &LoginAlert{
		Alert:      shouldWarnLoginFails(len(rows)),
		Window:     loginAlertWindow,
		MinFails:   loginAlertMinFails,
		IPCount:    len(rows),
		FailCount:  total,
		IPs:        rows,
		ShareAlert: len(shares) > 0,
		Shares:     shares,
		MultiAlert: len(accounts) > 0,
		Accounts:   accounts,
	}, nil
}

func groupMultiLogins(rows []repository.ActiveSession, viewerSession string, limit int) []AccountLogins {
	if limit < 1 {
		limit = loginAlertIPLimit
	}
	order := make([]string, 0)
	byUser := make(map[string][]AccountSession)
	for _, row := range rows {
		name := strings.TrimSpace(row.Username)
		if name == "" || row.ID == "" {
			continue
		}
		if _, ok := byUser[name]; !ok {
			order = append(order, name)
		}
		byUser[name] = append(byUser[name], AccountSession{
			ID:       row.ID,
			IP:       strings.TrimSpace(row.IP),
			LastSeen: row.LastSeen,
			Current:  viewerSession != "" && row.ID == viewerSession,
		})
	}
	out := make([]AccountLogins, 0)
	for _, name := range order {
		sessions := dedupeSessions(byUser[name])
		if len(sessions) < loginMultiMinSession {
			continue
		}
		sortAccountSessions(sessions)
		item := AccountLogins{Username: name, SessionCount: len(sessions)}
		if len(sessions) > limit {
			sessions = sessions[:limit]
		}
		item.Sessions = sessions
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func dedupeSessions(in []AccountSession) []AccountSession {
	seen := make(map[string]int, len(in))
	out := make([]AccountSession, 0, len(in))
	for _, row := range in {
		idx, ok := seen[row.ID]
		if !ok {
			seen[row.ID] = len(out)
			out = append(out, row)
			continue
		}
		if row.LastSeen.After(out[idx].LastSeen) {
			out[idx] = row
		}
	}
	return out
}

func groupSharedLogins(rows []repository.ActiveLoginIP, viewerIP string, limit int) []SharedLogin {
	if limit < 1 {
		limit = loginAlertIPLimit
	}
	order := make([]string, 0)
	byUser := make(map[string]map[string]time.Time)
	for _, row := range rows {
		name := strings.TrimSpace(row.Username)
		ip := strings.TrimSpace(row.IP)
		if name == "" || ip == "" {
			continue
		}
		ips, ok := byUser[name]
		if !ok {
			ips = make(map[string]time.Time)
			byUser[name] = ips
			order = append(order, name)
		}
		if prev, seen := ips[ip]; !seen || row.LastSeen.After(prev) {
			ips[ip] = row.LastSeen
		}
	}
	out := make([]SharedLogin, 0)
	for _, name := range order {
		ips := byUser[name]
		if len(ips) < loginShareMinIPs {
			continue
		}
		sorted := make([]SharedLoginIP, 0, len(ips))
		for ip, seen := range ips {
			sorted = append(sorted, SharedLoginIP{IP: ip, LastSeen: seen, Current: viewerIP != "" && ip == viewerIP})
		}
		sortSharedIPs(sorted)
		item := SharedLogin{Username: name, IPCount: len(sorted)}
		if len(sorted) > limit {
			sorted = sorted[:limit]
		}
		item.IPs = sorted
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func sortSharedIPs(ips []SharedLoginIP) {
	for i := 1; i < len(ips); i++ {
		j := i
		for j > 0 && sharedIPLess(ips[j], ips[j-1]) {
			ips[j], ips[j-1] = ips[j-1], ips[j]
			j--
		}
	}
}

func sortAccountSessions(list []AccountSession) {
	for i := 1; i < len(list); i++ {
		j := i
		for j > 0 && accountSessionLess(list[j], list[j-1]) {
			list[j], list[j-1] = list[j-1], list[j]
			j--
		}
	}
}

func accountSessionLess(a, b AccountSession) bool {
	if a.LastSeen.Equal(b.LastSeen) {
		return a.ID < b.ID
	}
	return a.LastSeen.After(b.LastSeen)
}

func sharedIPLess(a, b SharedLoginIP) bool {
	if a.LastSeen.Equal(b.LastSeen) {
		return a.IP < b.IP
	}
	return a.LastSeen.After(b.LastSeen)
}

func (s *Service) loginBlock(ctx context.Context, ip string) (string, error) {
	if ip == "" {
		return "", nil
	}
	g, err := s.store.GetGuard(ctx, "ip:"+ip)
	if err != nil {
		return "", err
	}
	if g.LockedUntil.Valid && g.LockedUntil.Time.After(time.Now()) {
		return "ip", nil
	}
	return "", nil
}

func (s *Service) failLogin(ctx context.Context, ip string) {
	if ip == "" {
		s.log.Warn("admin login failure without ip")
		return
	}
	s.advanceGuard(ctx, "ip:"+ip, loginIPLockAfter, loginIPLockFor)
}

func (s *Service) advanceGuard(ctx context.Context, key string, after int, lockFor time.Duration) {
	g, err := s.store.GetGuard(ctx, key)
	if err != nil {
		s.log.Error("get guard failed", "key", key, "err", err)
		return
	}
	now := time.Now()
	next := nextGuardCount(g.FailCount, g.LockedUntil.Time, g.LockedUntil.Valid, now)
	if next >= after {
		until := now.Add(lockFor)
		if err = s.store.SaveGuard(ctx, key, next, &until); err != nil {
			s.log.Error("lock guard failed", "key", key, "fails", next, "until", until.Format(time.RFC3339), "err", err)
			return
		}
		s.log.Warn("admin login guard locked", "key", key, "fails", next, "until", until.Format(time.RFC3339))
		return
	}
	if err = s.store.SaveGuard(ctx, key, next, nil); err != nil {
		s.log.Error("bump guard failed", "key", key, "fails", next, "err", err)
	}
}

func nextGuardCount(failCount int, lockedUntil time.Time, hasLock bool, now time.Time) int {
	if hasLock && !lockedUntil.After(now) {
		return 1
	}
	return failCount + 1
}

func randomCode(n int) string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func renderCaptcha(code string) image.Image {
	w, h := 140, 48
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{240, 244, 250, 255}}, image.Point{}, draw.Src)
	for i := 0; i < 8; i++ {
		y := 4 + i*5
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{200, 210, 220, 255})
		}
	}
	ink := color.RGBA{20, 40, 70, 255}
	for i, r := range code {
		drawGlyph(img, 16+i*22, 14, r, ink)
	}
	return img
}

func drawGlyph(img *image.RGBA, ox, oy int, r rune, ink color.RGBA) {
	bits, ok := glyph5x7[r]
	if !ok {
		bits = glyph5x7['?']
	}
	for row := 0; row < 7; row++ {
		rowBits := bits[row]
		for col := 0; col < 5; col++ {
			if rowBits&(1<<uint(4-col)) == 0 {
				continue
			}
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					img.Set(ox+col*2+dx, oy+row*2+dy, ink)
				}
			}
		}
	}
}

// 5x7 点阵，避免依赖 golang.org/x/image。
var glyph5x7 = map[rune][7]uint8{
	'2': {0x0E, 0x11, 0x01, 0x06, 0x08, 0x10, 0x1F},
	'3': {0x0E, 0x11, 0x01, 0x06, 0x01, 0x11, 0x0E},
	'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5': {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
	'6': {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C},
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'B': {0x1E, 0x11, 0x11, 0x1E, 0x11, 0x11, 0x1E},
	'C': {0x0E, 0x11, 0x10, 0x10, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x10, 0x1E, 0x10, 0x10, 0x10},
	'G': {0x0E, 0x11, 0x10, 0x17, 0x11, 0x11, 0x0E},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'J': {0x01, 0x01, 0x01, 0x01, 0x11, 0x11, 0x0E},
	'K': {0x11, 0x12, 0x14, 0x18, 0x14, 0x12, 0x11},
	'L': {0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x15, 0x11, 0x11, 0x11},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'P': {0x1E, 0x11, 0x11, 0x1E, 0x10, 0x10, 0x10},
	'Q': {0x0E, 0x11, 0x11, 0x11, 0x15, 0x12, 0x0D},
	'R': {0x1E, 0x11, 0x11, 0x1E, 0x14, 0x12, 0x11},
	'S': {0x0E, 0x11, 0x10, 0x0E, 0x01, 0x11, 0x0E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'V': {0x11, 0x11, 0x11, 0x11, 0x11, 0x0A, 0x04},
	'W': {0x11, 0x11, 0x11, 0x15, 0x15, 0x1B, 0x11},
	'X': {0x11, 0x11, 0x0A, 0x04, 0x0A, 0x11, 0x11},
	'Y': {0x11, 0x11, 0x0A, 0x04, 0x04, 0x04, 0x04},
	'Z': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x10, 0x1F},
	'?': {0x0E, 0x11, 0x01, 0x02, 0x04, 0x00, 0x04},
}
