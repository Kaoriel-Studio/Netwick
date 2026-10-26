package report

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kaoriel-studio/netwick/internal/checks"
	"github.com/kaoriel-studio/netwick/internal/platform/fake"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

func TestSanitize(t *testing.T) {
	s := Sanitizer{UserProfile: `C:\Users\Ivan`, UserName: "Ivan", Computer: "IVAN-PC", AllowIPs: map[string]bool{"104.16.120.50": true}}
	cases := []struct{ in, want string }{
		{`C:\Users\Ivan\AppData\Local\Riot Games`, `%USERPROFILE%\AppData\Local\Riot Games`},
		{`c:/users/ivan/Desktop`, `%USERPROFILE%/Desktop`},
		{`D:\Users\Someone Else\x`, `%USERPROFILE% Else\x`},
		{`E:\Users\Petr\Logs`, `%USERPROFILE%\Logs`},
		{"user Ivan logged in", "user <user> logged in"},
		{"Ivanov stays", "Ivanov stays"},
		{"host IVAN-PC online", "host <pc> online"},
		{"mail ivan.petrov@mail.ru now", "mail <email> now"},
		{"t=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc", "t=<token>"},
		{"puuid 3f2b8a10-1c2d-4e5f-8a9b-0c1d2e3f4a5b", "puuid <id>"},
		{"key 0123456789abcdef0123456789abcdef01", "key <token>"},
		{"access_token=abc.def&x=1", "access_token=<hidden>&x=1"},
		{"ip 192.168.1.15 and 104.16.120.50 and 8.8.8.8", "ip <ip> and 104.16.120.50 and 8.8.8.8"},
		{"public 95.31.2.17", "public <ip>"},
		{"v6 fe80::1c2b:3a4d:5e6f:7a8b", "v6 <ip>"},
		{"Player PlayerOne#EUW1 joined", "Player <riot-id> joined"},
		{"Windows 11 Pro 24H2 build 26100", "Windows 11 Pro 24H2 build 26100"},
		{"valorant.secure.dyn.riotcdn.net", "valorant.secure.dyn.riotcdn.net"},
	}
	for _, c := range cases {
		if got := s.String(c.in); got != c.want {
			t.Errorf("%q\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestReportHasNoPersonalData(t *testing.T) {
	sys := fake.New(fake.OwnerRealRun)
	res := checks.NewRunner(checks.All()).Run(context.Background(), sys)
	v := verdict.Overall(res, "")
	s := Sanitizer{UserProfile: sys.Env("USERPROFILE"), UserName: sys.Env("USERNAME"), Computer: sys.Env("COMPUTERNAME"), AllowIPs: AllowIPsFrom(res)}
	rep := Build("Netwick", "test", "demo", time.Now(), res, v, nil, s)
	b, _ := json.Marshal(rep)
	out := string(b)
	for _, bad := range []string{`Users\\Player`, "PLAYER-PC", "Player\\\\"} {
		if strings.Contains(out, bad) {
			t.Errorf("report leaks %q", bad)
		}
	}
	if !strings.Contains(out, "%USERPROFILE%") && !strings.Contains(out, "E:\\\\Riot Games") {
		t.Error("expected install paths")
	}
	if !strings.Contains(out, "104.16.120.50") {
		t.Error("riot IP should stay")
	}
}

func TestSanitizerUserNameInsideReplacement(t *testing.T) {
	s := Sanitizer{UserName: "user", Computer: "PC-user"}
	done := make(chan string, 1)
	go func() { done <- s.String("owner user and user, host PC-user; superuser keeps") }()
	select {
	case got := <-done:
		want := "owner <user> and <user>, host <pc>; superuser keeps"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sanitizer hangs")
	}
}
