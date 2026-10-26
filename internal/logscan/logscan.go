package logscan

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"github.com/kaoriel-studio/netwick/internal/probe"
)

const MaxBytes = 5 << 20

var errorRe = regexp.MustCompile(`SSL failure|Zero certificates|Failure to connect to URL|Connection was reset|timed out`)
var hostRe = regexp.MustCompile(`https?://([a-zA-Z0-9.-]+)`)

var ErrorTypes = map[string]string{
	"SSL failure":               "ssl",
	"Zero certificates":         "certificates",
	"Failure to connect to URL": "connect",
	"Connection was reset":      "reset",
	"timed out":                 "timeout",
}

type Summary struct {
	Files     int            `json:"files"`
	Incidents int            `json:"incidents"`
	Hosts     map[string]int `json:"hosts"`
	Types     map[string]int `json:"types"`
}

func NewSummary() Summary {
	return Summary{Hosts: map[string]int{}, Types: map[string]int{}}
}

func (s *Summary) Scan(data []byte) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		m := errorRe.FindString(line)
		if m == "" {
			continue
		}
		s.Incidents++
		s.Types[ErrorTypes[m]]++
		if hm := hostRe.FindStringSubmatch(line); hm != nil {
			h := strings.ToLower(strings.TrimSuffix(hm[1], "."))
			if probe.IsRiotHost(h) {
				s.Hosts[h]++
			}
		}
	}
}
