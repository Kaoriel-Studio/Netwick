package riotstatus

import (
	"encoding/json"
	"strings"
)

var Regions = []string{"eu", "na", "ap", "kr", "br", "latam"}

const DefaultRegion = "eu"

type Product struct {
	ID       string
	Template string
}

var Products = []Product{
	{ID: "valorant", Template: "https://valorant.secure.dyn.riotcdn.net/channels/public/x/status/%s.json"},
	{ID: "riot_client", Template: "https://riot-client.dyn.riotcdn.net/channels/public/x/status/%s.json"},
}

const MaxBody = 1 << 20

func ValidRegion(r string) bool {
	for _, x := range Regions {
		if x == r {
			return true
		}
	}
	return false
}

func URL(p Product, region string) string {
	if !ValidRegion(region) {
		region = DefaultRegion
	}
	return strings.Replace(p.Template, "%s", region, 1)
}

func ValidURL(u string) bool {
	for _, p := range Products {
		for _, r := range Regions {
			if u == URL(p, r) {
				return true
			}
		}
	}
	return false
}

type Item struct {
	TitleEN string `json:"titleEn"`
	TitleRU string `json:"titleRu"`
}

type Status struct {
	Product      string `json:"product"`
	Incidents    []Item `json:"incidents"`
	Maintenances []Item `json:"maintenances"`
}

func (s Status) Clear() bool { return len(s.Incidents) == 0 && len(s.Maintenances) == 0 }

type rawTitle struct {
	Locale  string `json:"locale"`
	Content string `json:"content"`
}

type rawEvent struct {
	Titles []rawTitle `json:"titles"`
}

type rawStatus struct {
	Incidents    []rawEvent `json:"incidents"`
	Maintenances []rawEvent `json:"maintenances"`
}

func clean(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 160 {
		s = string(r[:160]) + "…"
	}
	return s
}

func item(e rawEvent) Item {
	var it Item
	for _, t := range e.Titles {
		switch strings.ToLower(t.Locale) {
		case "en_us":
			it.TitleEN = clean(t.Content)
		case "ru_ru":
			it.TitleRU = clean(t.Content)
		}
	}
	if it.TitleEN == "" && len(e.Titles) > 0 {
		it.TitleEN = clean(e.Titles[0].Content)
	}
	if it.TitleRU == "" {
		it.TitleRU = it.TitleEN
	}
	return it
}

func Parse(product string, b []byte) (Status, error) {
	var raw rawStatus
	if err := json.Unmarshal(b, &raw); err != nil {
		return Status{}, err
	}
	st := Status{Product: product, Incidents: []Item{}, Maintenances: []Item{}}
	for _, e := range raw.Incidents {
		st.Incidents = append(st.Incidents, item(e))
	}
	for _, e := range raw.Maintenances {
		st.Maintenances = append(st.Maintenances, item(e))
	}
	return st, nil
}
