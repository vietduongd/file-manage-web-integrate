package config

import "testing"

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
