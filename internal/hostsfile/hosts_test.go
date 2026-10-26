package hostsfile

import (
	"strings"
	"testing"
)

const base = "# sample\r\n127.0.0.1 localhost\r\n10.0.0.5 nas.home\r\n"

func TestParseAndClean(t *testing.T) {
	in := base +
		"# >>> NETWICK >>> (managed routes)\r\n65.9.46.11 valorant.secure.dyn.riotcdn.net\r\n# <<< NETWICK <<<\r\n" +
		"# >>> CYPHERLINK-CDN >>>\r\n104.16.120.50 auth.riotgames.com\r\n1.2.3.4 my.custom.host\r\n# <<< CYPHERLINK-CDN <<<\r\n" +
		"# RIOT-TLS-FIX BEGIN\r\n104.18.157.37 clientconfig.rpg.riotgames.com\r\n# RIOT-TLS-FIX END\r\n" +
		"104.16.56.21 entitlements.auth.riotgames.com\r\n" +
		"# 1.1.1.1 auth.riotgames.com\r\n" +
		"5.5.5.5 notriotgames.com\r\n6.6.6.6 riotgames.com.evil.com\r\n"
	p := Parse([]byte(in))
	if len(p.Sections) != 3 {
		t.Fatalf("sections %v", p.Sections)
	}
	if len(p.RiotEntries) != 4 || p.Pinned["auth.riotgames.com"] != "104.16.120.50" || p.Pinned["entitlements.auth.riotgames.com"] != "104.16.56.21" {
		t.Fatalf("entries %+v", p.RiotEntries)
	}
	out, removed, changed := Clean([]byte(in))
	if !changed || len(removed) == 0 {
		t.Fatal("expected change")
	}
	s := string(out)
	for _, keep := range []string{"127.0.0.1 localhost", "10.0.0.5 nas.home", "# 1.1.1.1 auth.riotgames.com", "5.5.5.5 notriotgames.com", "6.6.6.6 riotgames.com.evil.com", "# sample"} {
		if !strings.Contains(s, keep) {
			t.Errorf("foreign line removed: %q", keep)
		}
	}
	for _, gone := range []string{"NETWICK", "CYPHERLINK", "RIOT-TLS-FIX", "65.9.46.11", "104.16.120.50", "104.18.157.37", "104.16.56.21"} {
		if strings.Contains(s, gone) {
			t.Errorf("left %q", gone)
		}
	}
	if !strings.Contains(s, "\r\n") || strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Error("line endings not preserved")
	}
	out2, _, changed2 := Clean(out)
	if changed2 || string(out2) != s {
		t.Error("clean is not idempotent")
	}
	if Parse(out).HasLegacy() {
		t.Error("still legacy after clean")
	}
	restored := Restore(out, removed)
	if !Parse(restored).HasLegacy() {
		t.Error("restore lost entries")
	}
}

func TestUntouched(t *testing.T) {
	out, removed, changed := Clean([]byte(base))
	if changed || removed != nil || string(out) != base {
		t.Fatal("clean hosts changed")
	}
	if Parse([]byte("# Netwick is great\n1.2.3.4 a.b\n")).HasLegacy() {
		t.Error("plain comment treated as section")
	}
}
