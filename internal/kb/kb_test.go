package kb

import (
	"net/url"
	"testing"

	"github.com/kaoriel-studio/netwick/internal/fixes"
)

func db(t *testing.T) *DB {
	t.Helper()
	d, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestIntegrity(t *testing.T) {
	d := db(t)
	if len(d.Entries) < 90 {
		t.Fatalf("only %d entries", len(d.Entries))
	}
	seen := map[string]bool{}
	for _, e := range d.Entries {
		if seen[e.Code] {
			t.Errorf("duplicate %s", e.Code)
		}
		seen[e.Code] = true
		u, err := url.Parse(e.SourceURL)
		if err != nil || u.Scheme != "https" || (u.Host != "support.riotgames.com" && u.Host != "support.microsoft.com") {
			t.Errorf("%s: bad sourceUrl %q", e.Code, e.SourceURL)
		}
		if e.Title.RU == "" || e.Title.EN == "" || e.Explain.RU == "" || e.Explain.EN == "" {
			t.Errorf("%s: missing text", e.Code)
		}
		if len(e.Steps) == 0 {
			t.Errorf("%s: no steps", e.Code)
		}
		for _, s := range e.Steps {
			if !fixes.StepExists(s) {
				t.Errorf("%s: unknown step %s/%s", e.Code, s.Kind, s.ID)
			}
		}
		if e.VerifiedAt == "" {
			t.Errorf("%s: no verifiedAt", e.Code)
		}
	}
	for _, code := range []string{"VAL 43", "VAL 29", "VAN 9001", "VAN 9002", "VAN 9006", "VAN 9101", "VAN -81", "VAN 68", "VAN 57", "VAN 128", "VAN 185", "VAN 152", "VAN: RESTRICTION", "VAN: RESTRICTION: 1", "VAN: Incompatible Software", "VAN: Incompatible OEM Driver", "VAN: STATUS_VMD_9001", "VAN: STATUS_SB_POLICY"} {
		e, ok := d.Get(code)
		if !ok || !e.Ported {
			t.Errorf("%s must be ported", code)
		}
	}
}

func TestOrderOfVAN68(t *testing.T) {
	e, _ := db(t).Get("VAN 68")
	want := []string{"reboot", "C03", "F07", "G01", "G02", "G11", "F09", "G10", "official"}
	for i, s := range e.Steps {
		if i >= len(want) || s.ID != want[i] {
			t.Fatalf("steps %v", e.Steps)
		}
	}
}

func TestSearch(t *testing.T) {
	d := db(t)
	cases := []struct {
		q    string
		want []string
	}{
		{"68", []string{"VAN 68", "VAL 68"}},
		{"van68", []string{"VAN 68"}},
		{"VAN 68", []string{"VAN 68"}},
		{"VAN-68", []string{"VAN 68"}},
		{"VAN-81", []string{"VAN -81"}},
		{"van -81", []string{"VAN -81"}},
		{"van 81", []string{"VAN -81"}},
		{"val 43", []string{"VAL 43"}},
		{"43", []string{"VAL 43"}},
		{"VAN: RESTRICTION: 3", []string{"VAN: RESTRICTION: 3"}},
		{"restriction 1", []string{"VAN: RESTRICTION: 1"}},
		{"VAN: RESTRICTION: 1", []string{"VAN: RESTRICTION: 1"}},
		{"VAN 9001", []string{"VAN 9001"}},
		{"status_vmd_9001", []string{"VAN: STATUS_VMD_9001"}},
		{"incompatible software", []string{"VAN: Incompatible Software"}},
		{"VAN Missing -117", []string{"VAN Missing -117"}},
		{"0%", []string{"UPDATE 0%"}},
		{"пинг", []string{"CONNECTION"}},
	}
	for _, c := range cases {
		got := d.Search(c.q)
		if len(got) < len(c.want) {
			t.Errorf("%q: got %d results", c.q, len(got))
			continue
		}
		for i, w := range c.want {
			if got[i].Entry.Code != w {
				t.Errorf("%q: result %d = %s, want %s", c.q, i, got[i].Entry.Code, w)
			}
		}
	}
	for _, g := range []string{"", "   ", "asdfgh", "<script>", "DROP TABLE", "99999", string(make([]byte, 200))} {
		if r := d.Search(g); len(r) != 0 {
			t.Errorf("%q: expected nothing, got %s", g, r[0].Entry.Code)
		}
	}
}
