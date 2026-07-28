package auth

import (
	"testing"
	"time"
)

const testSecret = "test-secret-for-jwt"

func mustTokenPair(t *testing.T) *TokenPair {
	t.Helper()
	pair, err := GenerateTokenPair("admin", testSecret, time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	return pair
}

func TestParseAccessTokenAcceptsAccessToken(t *testing.T) {
	pair := mustTokenPair(t)

	claims, err := ParseAccessToken(pair.AccessToken, testSecret)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.Username != "admin" {
		t.Fatalf("Username = %q, muốn %q", claims.Username, "admin")
	}
}

func TestParseRefreshTokenAcceptsRefreshToken(t *testing.T) {
	pair := mustTokenPair(t)

	username, err := ParseRefreshToken(pair.RefreshToken, testSecret)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if username != "admin" {
		t.Fatalf("username = %q, muốn %q", username, "admin")
	}
}

// Hai token ký cùng secret nên chữ ký và hạn đều hợp lệ chéo nhau.
// Thứ duy nhất phân biệt chúng là claim Issuer — phải kiểm nó, nếu không
// hai loại token dùng thay nhau được và TTL riêng của từng loại mất ý nghĩa.

func TestParseRefreshTokenRejectsAccessToken(t *testing.T) {
	pair := mustTokenPair(t)

	if _, err := ParseRefreshToken(pair.AccessToken, testSecret); err == nil {
		t.Fatal("access token không được dùng thay refresh token")
	}
}

func TestParseAccessTokenRejectsRefreshToken(t *testing.T) {
	pair := mustTokenPair(t)

	// Nguy hiểm hơn chiều kia: refresh token sống 168h, mà claims của nó
	// không có username nên sẽ lọt qua middleware với username rỗng.
	if _, err := ParseAccessToken(pair.RefreshToken, testSecret); err == nil {
		t.Fatal("refresh token không được dùng thay access token")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	pair := mustTokenPair(t)

	if _, err := ParseAccessToken(pair.AccessToken, "secret-khac"); err == nil {
		t.Fatal("access token ký secret khác phải bị từ chối")
	}
	if _, err := ParseRefreshToken(pair.RefreshToken, "secret-khac"); err == nil {
		t.Fatal("refresh token ký secret khác phải bị từ chối")
	}
}

func TestParseRejectsExpiredTokens(t *testing.T) {
	expired, err := GenerateTokenPair("admin", testSecret, -time.Minute, -time.Minute)
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}

	if _, err := ParseAccessToken(expired.AccessToken, testSecret); err == nil {
		t.Fatal("access token hết hạn phải bị từ chối")
	}
	if _, err := ParseRefreshToken(expired.RefreshToken, testSecret); err == nil {
		t.Fatal("refresh token hết hạn phải bị từ chối")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := ParseAccessToken("khong-phai-jwt", testSecret); err == nil {
		t.Fatal("chuỗi rác phải bị từ chối")
	}
	if _, err := ParseRefreshToken("khong-phai-jwt", testSecret); err == nil {
		t.Fatal("chuỗi rác phải bị từ chối")
	}
}
