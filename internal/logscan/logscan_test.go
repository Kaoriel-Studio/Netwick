package logscan

import "testing"

func TestScan(t *testing.T) {
	log := `2026| WARN| Failure to connect to URL https://data.riotgames.com/x: SSL failure
2026| WARN| Connection was reset https://auth.riotgames.com/token?access_token=eyJhbGciOi
2026| WARN| timed out https://notriotgames.com/x
2026| WARN| timed out https://riotgames.com.evil.com/x
2026| WARN| Zero certificates https://cdn.example.com
2026| INFO| fetched https://auth.riotgames.com/ok
2026| WARN| Failure to connect to URL https://DATA.RIOTGAMES.COM/y
`
	s := NewSummary()
	s.Scan([]byte(log))
	if s.Incidents != 6 {
		t.Errorf("incidents %d", s.Incidents)
	}
	if s.Hosts["data.riotgames.com"] != 2 || s.Hosts["auth.riotgames.com"] != 1 || len(s.Hosts) != 2 {
		t.Errorf("hosts %v", s.Hosts)
	}
}
