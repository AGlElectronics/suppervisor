package config

import "testing"

func TestProductionRejectsAuthBypass(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_DEV_BYPASS", "true")
	t.Setenv("OIDC_ISSUER", "https://id.example.test/realms/home")
	if _, err := Load(); err == nil {
		t.Fatal("expected production auth bypass to be rejected")
	}
}

func TestTelegramRequiresAllowlist(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("TELEGRAM_BOT_TOKEN", "secret")
	t.Setenv("TELEGRAM_ALLOWED_USER_IDS", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing Telegram allowlist to be rejected")
	}
}

func TestProductionDisablesDemoData(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_DEV_BYPASS", "false")
	t.Setenv("OIDC_ISSUER", "https://id.example.test/realms/home")
	t.Setenv("DEV_DEMO_DATA", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DevDemoData {
		t.Fatal("production must not seed demo data")
	}
}
