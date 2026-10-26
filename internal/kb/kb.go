package kb

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/kaoriel-studio/netwick/internal/core"
	kbdata "github.com/kaoriel-studio/netwick/kb"
)

type Text struct {
	RU string `json:"ru"`
	EN string `json:"en"`
}

type Entry struct {
	Code       string      `json:"code"`
	Aliases    []string    `json:"aliases"`
	Family     string      `json:"family"`
	Title      Text        `json:"title"`
	Explain    Text        `json:"explain"`
	Steps      []core.Step `json:"steps"`
	SourceURL  string      `json:"sourceUrl"`
	VerifiedAt string      `json:"verifiedAt"`
	Ported     bool        `json:"ported"`
}

type DB struct {
	Entries []Entry
	index   map[string][]int
}

var (
	once    sync.Once
	defDB   *DB
	loadErr error
)

func Default() (*DB, error) {
	once.Do(func() {
		defDB, loadErr = Load(kbdata.ErrorsJSON)
	})
	return defDB, loadErr
}

func Load(b []byte) (*DB, error) {
	var entries []Entry
	if err := json.Unmarshal(b, &entries); err != nil {
		return nil, err
	}
	db := &DB{Entries: entries, index: map[string][]int{}}
	for i, e := range entries {
		keys := map[string]bool{Compact(e.Code): true}
		for _, a := range e.Aliases {
			keys[Compact(a)] = true
		}
		if f, n, ok := numeric(e.Code); ok {
			keys[f+n] = true
		}
		for k := range keys {
			if k != "" {
				db.index[k] = append(db.index[k], i)
			}
		}
	}
	return db, nil
}

func Compact(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '%' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var numRe = regexp.MustCompile(`^\s*(VAN|VAL)?\s*[:#]?\s*(-|–|—)?\s*(\d{1,5})\s*$`)

func numeric(s string) (family, number string, ok bool) {
	m := numRe.FindStringSubmatch(strings.ToUpper(s))
	if m == nil {
		return "", "", false
	}
	n := m[3]
	if m[2] != "" {
		n = "-" + n
	}
	return m[1], n, true
}

const MaxQuery = 64

type Match struct {
	Entry Entry  `json:"entry"`
	Exact bool   `json:"exact"`
	Query string `json:"query"`
}

func (db *DB) Get(code string) (Entry, bool) {
	for _, e := range db.Entries {
		if e.Code == code {
			return e, true
		}
	}
	return Entry{}, false
}

func (db *DB) Search(q string) []Match {
	q = strings.TrimSpace(q)
	if q == "" || len([]rune(q)) > MaxQuery {
		return []Match{}
	}
	var order []int
	seen := map[int]bool{}
	add := func(idx []int) {
		for _, i := range idx {
			if !seen[i] {
				seen[i] = true
				order = append(order, i)
			}
		}
	}
	if fam, n, ok := numeric(q); ok {
		alt := strings.TrimPrefix(n, "-")
		if !strings.HasPrefix(n, "-") {
			alt = "-" + n
		}
		fams := []string{fam}
		if fam == "" {
			fams = []string{"VAN", "VAL"}
		}
		for _, f := range fams {
			add(db.index[f+n])
		}
		if len(order) == 0 {
			for _, f := range fams {
				add(db.index[f+alt])
			}
		}
	}
	c := Compact(q)
	if len(order) == 0 && c != "" {
		add(db.index[c])
		if len(order) == 0 {
			add(db.index["VAN"+c])
			add(db.index["VAL"+c])
		}
	}
	if len(order) == 0 && len([]rune(c)) >= 4 {
		var keys []string
		for k := range db.index {
			if strings.Contains(k, c) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(db.index[k])
		}
	}
	out := make([]Match, 0, len(order))
	for _, i := range order {
		out = append(out, Match{Entry: db.Entries[i], Exact: Compact(db.Entries[i].Code) == c, Query: q})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Exact != out[j].Exact {
			return out[i].Exact
		}
		return out[i].Entry.Family == "VAN" && out[j].Entry.Family != "VAN"
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
