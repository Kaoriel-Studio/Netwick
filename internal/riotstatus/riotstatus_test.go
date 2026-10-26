package riotstatus

import "testing"

func TestValidURL(t *testing.T) {
	if !ValidURL(URL(Products[0], "eu")) || !ValidURL(URL(Products[1], "latam")) {
		t.Fatal("own URLs must be valid")
	}
	for _, u := range []string{"", "https://valorant.secure.dyn.riotcdn.net/channels/public/x/status/xx.json", "https://example.com/eu.json", URL(Products[0], "eu") + "?a=1"} {
		if ValidURL(u) {
			t.Errorf("%q must be rejected", u)
		}
	}
	if URL(Products[0], "../../x") != URL(Products[0], DefaultRegion) {
		t.Error("bad region must fall back to default")
	}
}

func TestParse(t *testing.T) {
	b := []byte(`{"id":"EU","incidents":[{"titles":[{"locale":"en_US","content":"Login  issues"},{"locale":"ru_RU","content":"Проблемы со входом"}]}],"maintenances":[{"titles":[{"locale":"de_DE","content":"Wartung"}]}]}`)
	st, err := Parse("valorant", b)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Incidents) != 1 || st.Incidents[0].TitleEN != "Login issues" || st.Incidents[0].TitleRU != "Проблемы со входом" {
		t.Fatalf("incidents %+v", st.Incidents)
	}
	if len(st.Maintenances) != 1 || st.Maintenances[0].TitleEN != "Wartung" || st.Maintenances[0].TitleRU != "Wartung" {
		t.Fatalf("maintenances %+v", st.Maintenances)
	}
	if st.Clear() {
		t.Fatal("not clear")
	}
	if _, err := Parse("valorant", []byte("<html>")); err == nil {
		t.Fatal("expected error")
	}
}
