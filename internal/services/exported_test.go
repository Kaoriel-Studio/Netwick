package services

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

func TestExportedMethodsDocumented(t *testing.T) {
	b, err := os.ReadFile("../../docs/SECURITY.md")
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile("(?m)^- `(\\w+\\.\\w+)`$").FindAllStringSubmatch(string(b), -1) {
		documented[m[1]] = true
	}
	types := []any{&AppService{}, &CheckService{}, &FixService{}, &KBService{}, &ReportService{}, &ToolsService{}}
	var actual []string
	for _, v := range types {
		rt := reflect.TypeOf(v)
		name := rt.Elem().Name()
		for i := 0; i < rt.NumMethod(); i++ {
			actual = append(actual, name+"."+rt.Method(i).Name)
		}
	}
	sort.Strings(actual)
	for _, m := range actual {
		if !documented[m] {
			t.Errorf("exported method %s is not listed in docs/SECURITY.md", m)
		}
		delete(documented, m)
	}
	for m := range documented {
		t.Errorf("docs/SECURITY.md lists %s, which no longer exists", m)
	}
}
