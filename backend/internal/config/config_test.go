package config

import (
	"reflect"
	"testing"
)

func TestLoadReadsEmbedTicketConfig(t *testing.T) {
	t.Setenv("EMBED_TICKET_VERIFY_URL", "https://tecs.vn/v1/api/manage/embed-ticket/verify")
	t.Setenv("EMBED_TICKET_SERVICE_KEY", "file-manage-iam-service-key-2026")

	cfg := Load()

	if cfg.EmbedTicketVerifyURL != "https://tecs.vn/v1/api/manage/embed-ticket/verify" {
		t.Fatalf("EmbedTicketVerifyURL = %q", cfg.EmbedTicketVerifyURL)
	}
	if cfg.EmbedTicketServiceKey != "file-manage-iam-service-key-2026" {
		t.Fatalf("EmbedTicketServiceKey = %q", cfg.EmbedTicketServiceKey)
	}
}

func TestLoadProductionRejectsDefaultSecrets(t *testing.T) {
	t.Setenv("SERVER_ENV", "production")
	t.Setenv("JWT_SECRET", "change-me-in-production")
	t.Setenv("ADMIN_PASSWORD", "admin123")

	if _, err := LoadValidated(); err == nil {
		t.Fatal("LoadValidated succeeded with production default secrets, want error")
	}
}

func TestConfigDoesNotExposeLegacyExternalAuthSettings(t *testing.T) {
	cfgType := reflect.TypeOf(Config{})
	for _, field := range []string{
		"ExternalAuthVerifyURL",
		"ExternalAuthVerifyMethod",
		"ExternalAuthAppID",
		"ExternalAuthAPIKey",
	} {
		if _, ok := cfgType.FieldByName(field); ok {
			t.Fatalf("legacy external auth field %s should be removed", field)
		}
	}
}
