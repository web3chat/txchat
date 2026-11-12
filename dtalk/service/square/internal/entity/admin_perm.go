package entity

import "strings"

const (
	PermOfficialBan = "official_ban"
	PermGold        = "gold"
	PermSettings    = "settings"
	PermStaff       = "staff"
)

// PermSpec 运营台可分配权限。staff 只属于根管理员，不能下发。
type PermSpec struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// AssignablePerms 管理员可以分配或收回的权限，顺序即展示顺序。
var AssignablePerms = []PermSpec{
	{Code: PermOfficialBan, Label: "封禁公众号"},
	{Code: PermGold, Label: "金标"},
	{Code: PermSettings, Label: "广场开关与白名单"},
}

// DefaultPerms 按角色给出初始权限。staff 仅根管理员持有。
func DefaultPerms(role string, staff bool) []string {
	var out []string
	if role == RoleSuper {
		out = append(out, PermOfficialBan, PermGold, PermSettings)
	}
	if staff {
		out = append(out, PermStaff)
	}
	return out
}

// ParsePerms 解析逗号分隔的权限码，忽略空项并去重，保持目录顺序。
func ParsePerms(raw string) []string {
	want := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		code := strings.TrimSpace(part)
		if code == "" {
			continue
		}
		want[code] = struct{}{}
	}
	return orderPerms(want)
}

// FormatPerms 写成稳定的逗号分隔串。
func FormatPerms(perms []string) string {
	return strings.Join(ParsePerms(strings.Join(perms, ",")), ",")
}

// NormalizeAssignable 只保留可分配权限。含未知码或 staff 时拒绝。
func NormalizeAssignable(in []string) ([]string, bool) {
	want := map[string]struct{}{}
	for _, raw := range in {
		code := strings.TrimSpace(raw)
		if code == "" {
			continue
		}
		if !isAssignable(code) {
			return nil, false
		}
		want[code] = struct{}{}
	}
	return orderPerms(want), true
}

func (a *Admin) EffectivePerms() []string {
	if a == nil {
		return nil
	}
	if a.PermReady {
		return append([]string(nil), a.Permissions...)
	}
	return DefaultPerms(a.Role, false)
}

func HasPerm(perms []string, code string) bool {
	for _, p := range perms {
		if p == code {
			return true
		}
	}
	return false
}

func isAssignable(code string) bool {
	for _, spec := range AssignablePerms {
		if spec.Code == code {
			return true
		}
	}
	return false
}

func orderPerms(want map[string]struct{}) []string {
	known := make([]string, 0, len(want))
	seen := map[string]struct{}{}
	for _, spec := range AssignablePerms {
		if _, ok := want[spec.Code]; ok {
			known = append(known, spec.Code)
			seen[spec.Code] = struct{}{}
		}
	}
	if _, ok := want[PermStaff]; ok {
		known = append(known, PermStaff)
		seen[PermStaff] = struct{}{}
	}
	for code := range want {
		if _, ok := seen[code]; ok {
			continue
		}
		known = append(known, code)
	}
	return known
}
