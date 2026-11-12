package config

import "testing"

func TestIsProd(t *testing.T) {
	if !IsProd("prod") || !IsProd("production") || IsProd("local") {
		t.Fatalf("IsProd mismatch")
	}
}

func TestIsMockAllowed(t *testing.T) {
	if IsMockAllowed("prod", true) {
		t.Fatal("prod must deny mock")
	}
	if !IsMockAllowed("local", true) {
		t.Fatal("local+allow should permit mock")
	}
	if IsMockAllowed("local", false) {
		t.Fatal("allow_mock=false should deny")
	}
	if IsMockAllowed("staging", true) {
		t.Fatal("unknown env should deny mock")
	}
}

func TestPostgresAutoWhenHostUnset(t *testing.T) {
	t.Setenv("SQUARE_ENV", "local")
	t.Setenv("SQUARE_PG_HOST", "")
	t.Setenv("SQUARE_PG_PORT", "")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if !Conf.Postgres.Auto {
		t.Fatal("expected auto postgres")
	}
	if Conf.Postgres.Host != "127.0.0.1" || Conf.Postgres.Port != "15432" {
		t.Fatalf("auto dsn %s:%s", Conf.Postgres.Host, Conf.Postgres.Port)
	}
}

func TestPostgresExplicitHost(t *testing.T) {
	t.Setenv("SQUARE_ENV", "local")
	t.Setenv("SQUARE_PG_HOST", "db.internal")
	t.Setenv("SQUARE_PG_PORT", "")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if Conf.Postgres.Auto {
		t.Fatal("explicit host must not auto")
	}
	if Conf.Postgres.Host != "db.internal" || Conf.Postgres.Port != "5432" {
		t.Fatalf("explicit dsn %s:%s", Conf.Postgres.Host, Conf.Postgres.Port)
	}
}

func TestDeleteURL(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"http://example", "http://example/oss/delete"},
		{"http://example/oss", "http://example/oss/delete"},
		{"http://example/oss/upload", "http://example/oss/delete"},
		{"http://example/oss/delete", "http://example/oss/delete"},
	}
	for _, tc := range cases {
		if got := (OSSConfig{BaseURL: tc.in}).DeleteURL(); got != tc.want {
			t.Fatalf("in=%s got=%s want=%s", tc.in, got, tc.want)
		}
	}
}

func TestClampTTLDays(t *testing.T) {
	t.Parallel()
	if clampTTLDays(0) != 0 || clampTTLDays(-1) != 0 {
		t.Fatal("0 should disable")
	}
	if clampTTLDays(15) != 30 || clampTTLDays(100) != 62 || clampTTLDays(45) != 45 {
		t.Fatal("ttl clamp")
	}
}

func TestRetentionAndHotConfig(t *testing.T) {
	t.Setenv("SQUARE_ENV", "local")
	t.Setenv("SQUARE_PG_HOST", "127.0.0.1")
	t.Setenv("SQUARE_POST_TTL_DAYS", "40")
	t.Setenv("SQUARE_HOT_WINDOW_DAYS", "10")
	t.Setenv("SQUARE_OSS_DELETE", "false")
	t.Setenv("SQUARE_RETENTION_INTERVAL", "30m")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if Conf.Retention.TTLDays != 40 || Conf.HotWindowDays != 10 {
		t.Fatalf("ttl=%d hot=%d", Conf.Retention.TTLDays, Conf.HotWindowDays)
	}
	if Conf.OSS.DeleteEnabled {
		t.Fatal("oss delete should stay off")
	}
	if Conf.Retention.Interval.Minutes() != 30 {
		t.Fatalf("interval=%s", Conf.Retention.Interval)
	}
}

func TestParseBootstrapAdmins(t *testing.T) {
	t.Parallel()
	if parseBootstrapAdmins("  ") != nil {
		t.Fatal("blank should be nil")
	}
	got := parseBootstrapAdmins(" developer:change-me-please-12:super , operator:change-me-please-12:reviewer, broken, :pass:super ")
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].User != "developer" || got[0].Pass != "change-me-please-12" || got[0].Role != "super" {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].User != "operator" || got[1].Role != "reviewer" {
		t.Fatalf("second=%+v", got[1])
	}
}

func TestBootstrapAdminsFromEnv(t *testing.T) {
	t.Setenv("SQUARE_ENV", "local")
	t.Setenv("SQUARE_PG_HOST", "127.0.0.1")
	t.Setenv("SQUARE_BOOTSTRAP_ADMINS", "developer:change-me-please-12:super,operator:change-me-please-12:reviewer")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if len(Conf.Bootstrap.Accounts) != 2 || Conf.Bootstrap.Accounts[0].Role != "super" {
		t.Fatalf("accounts=%+v", Conf.Bootstrap.Accounts)
	}
}

func TestProdRequiresPostgresHost(t *testing.T) {
	t.Setenv("SQUARE_ENV", "prod")
	t.Setenv("SQUARE_JWT_SECRET", "12345678901234567890123456789012")
	t.Setenv("SQUARE_PG_HOST", "")
	if err := Init(); err == nil {
		t.Fatal("prod without SQUARE_PG_HOST must fail")
	}
}
