package handlers

import (
	"testing"
	"time"
)

// newTestLimiter trả về limiter với đồng hồ giả, để test được hành vi hết hạn
// cửa sổ mà không phải sleep.
func newTestLimiter(limit int, window time.Duration) (*loginRateLimiter, func(time.Duration)) {
	l := newLoginRateLimiter(limit, window)
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	advance := func(d time.Duration) { now = now.Add(d) }
	return l, advance
}

func TestNewLoginRateLimiterFallsBackOnNonsenseConfig(t *testing.T) {
	// LOGIN_RATE_LIMIT_MAX=0 không được hiểu thành "chặn tất cả",
	// và window=0 không được hiểu thành "hết hạn ngay lập tức".
	tests := []struct {
		name       string
		limit      int
		window     time.Duration
		wantLimit  int
		wantWindow time.Duration
	}{
		{"limit 0", 0, time.Minute, 5, time.Minute},
		{"limit âm", -3, time.Minute, 5, time.Minute},
		{"window 0", 3, 0, 3, 10 * time.Minute},
		{"window âm", 3, -time.Minute, 3, 10 * time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLoginRateLimiter(tc.limit, tc.window)
			if l.limit != tc.wantLimit {
				t.Errorf("limit = %d, muốn %d", l.limit, tc.wantLimit)
			}
			if l.window != tc.wantWindow {
				t.Errorf("window = %v, muốn %v", l.window, tc.wantWindow)
			}
		})
	}
}

func TestLimiterBlocksOnlyAfterReachingLimit(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if got := l.Check("10.0.0.1", "admin"); got.Limited {
			t.Fatalf("bị chặn quá sớm ở lần thử thứ %d", i+1)
		}
		l.AddFailure("10.0.0.1", "admin")
	}

	got := l.Check("10.0.0.1", "admin")
	if !got.Limited {
		t.Fatal("lần thứ 4 phải bị chặn")
	}
	if got.RetryAfter <= 0 {
		t.Fatalf("RetryAfter = %v, phải dương", got.RetryAfter)
	}
}

func TestLimiterReleasesAfterWindowExpires(t *testing.T) {
	l, advance := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.AddFailure("10.0.0.1", "admin")
	}
	if !l.Check("10.0.0.1", "admin").Limited {
		t.Fatal("phải đang bị chặn trước khi cửa sổ hết hạn")
	}

	advance(time.Minute + time.Second)

	if l.Check("10.0.0.1", "admin").Limited {
		t.Fatal("hết cửa sổ rồi thì phải cho thử lại")
	}
}

func TestLimiterStartsFreshWindowAfterExpiry(t *testing.T) {
	l, advance := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.AddFailure("10.0.0.1", "admin")
	}
	advance(time.Minute + time.Second)

	// Lần fail đầu tiên sau khi hết hạn phải mở cửa sổ mới đếm từ 1,
	// không được cộng dồn vào 3 lần cũ.
	l.AddFailure("10.0.0.1", "admin")
	if l.Check("10.0.0.1", "admin").Limited {
		t.Fatal("cửa sổ mới phải đếm lại từ đầu, không cộng dồn lần cũ")
	}
}

func TestLimiterRetryAfterShrinksAsWindowElapses(t *testing.T) {
	l, advance := newTestLimiter(1, 10*time.Minute)

	l.AddFailure("10.0.0.1", "admin")
	first := l.Check("10.0.0.1", "admin").RetryAfter

	advance(4 * time.Minute)
	second := l.Check("10.0.0.1", "admin").RetryAfter

	if second >= first {
		t.Fatalf("RetryAfter phải giảm dần: %v rồi %v", first, second)
	}
	if want := 6 * time.Minute; second != want {
		t.Fatalf("RetryAfter = %v, muốn %v", second, want)
	}
}

func TestLimiterResetClearsAttempts(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.AddFailure("10.0.0.1", "admin")
	}
	l.Reset("10.0.0.1", "admin")

	if l.Check("10.0.0.1", "admin").Limited {
		t.Fatal("Reset phải xoá sạch số lần fail")
	}
}

func TestLimiterIsolatesDifferentClients(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)

	for i := 0; i < 2; i++ {
		l.AddFailure("10.0.0.1", "admin")
	}

	if !l.Check("10.0.0.1", "admin").Limited {
		t.Fatal("client đã fail đủ phải bị chặn")
	}
	// Khoá gồm cả IP lẫn username, nên đổi một trong hai là bucket khác.
	if l.Check("10.0.0.2", "admin").Limited {
		t.Fatal("IP khác không được ăn theo giới hạn của IP đã fail")
	}
	if l.Check("10.0.0.1", "editor").Limited {
		t.Fatal("username khác không được ăn theo giới hạn")
	}
}

func TestLimiterNilIsNoop(t *testing.T) {
	// NewAuthHandler để limiter = nil khi LOGIN_RATE_LIMIT_DISABLED=true,
	// nên mọi method phải chịu được receiver nil.
	var l *loginRateLimiter

	if l.Check("10.0.0.1", "admin").Limited {
		t.Fatal("limiter nil không được chặn ai")
	}
	l.AddFailure("10.0.0.1", "admin") // không được panic
	l.Reset("10.0.0.1", "admin")      // không được panic
}

func TestLoginRateLimitKeyNormalisesUsername(t *testing.T) {
	// Username không phân biệt hoa thường và bỏ khoảng trắng thừa,
	// nếu không attacker chỉ cần đổi "Admin" thành "ADMIN" là có bucket mới.
	base := loginRateLimitKey("10.0.0.1", "admin")

	for _, variant := range []string{"ADMIN", "Admin", "  admin  ", "\tadmin\n"} {
		if got := loginRateLimitKey("10.0.0.1", variant); got != base {
			t.Errorf("loginRateLimitKey(ip, %q) = %q, muốn %q", variant, got, base)
		}
	}
}

func TestLoginRateLimitKeySeparatesIPAndUsername(t *testing.T) {
	if loginRateLimitKey("10.0.0.1", "admin") == loginRateLimitKey("10.0.0.2", "admin") {
		t.Fatal("IP khác nhau phải cho khoá khác nhau")
	}
	if loginRateLimitKey("10.0.0.1", "admin") == loginRateLimitKey("10.0.0.1", "editor") {
		t.Fatal("username khác nhau phải cho khoá khác nhau")
	}
}

func TestRetryAfterSecondsNeverReturnsZero(t *testing.T) {
	// Header Retry-After: 0 nghĩa là "thử lại ngay", đúng ngược ý định.
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "1"},
		{-5 * time.Second, "1"},
		{500 * time.Millisecond, "1"},
		{time.Second, "1"},
		{90 * time.Second, "90"},
		{1500 * time.Millisecond, "1"}, // cắt phần lẻ, không làm tròn lên
	}

	for _, tc := range tests {
		if got := retryAfterSeconds(tc.in); got != tc.want {
			t.Errorf("retryAfterSeconds(%v) = %q, muốn %q", tc.in, got, tc.want)
		}
	}
}
