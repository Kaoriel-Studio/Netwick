package hostsfile

import (
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/kaoriel-studio/netwick/internal/probe"
)

const ManagedMarker = "NETWICK"

var Markers = []string{"NETWICK", "CYPHERLINK-CDN", "CYPHERLINK-ROUTE", "RIOT-TLS-FIX"}

var markerRe = regexp.MustCompile(`(?i)^\s*#.*?\b(NETWICK|CYPHERLINK-CDN|CYPHERLINK-ROUTE|RIOT-TLS-FIX)\b`)
var closeRe = regexp.MustCompile(`(?i)<<<|\bEND\b`)
var openRe = regexp.MustCompile(`(?i)>>>|\bBEGIN\b|\bSTART\b`)

type Entry struct {
	IP      string `json:"ip"`
	Host    string `json:"host"`
	Line    int    `json:"line"`
	Section string `json:"section"`
}

type Parsed struct {
	Sections    []string          `json:"sections"`
	RiotEntries []Entry           `json:"riotEntries"`
	Pinned      map[string]string `json:"pinned"`
}

func (p Parsed) HasLegacy() bool {
	return len(p.Sections) > 0 || len(p.RiotEntries) > 0
}

func splitLines(content []byte) ([]string, string) {
	s := string(content)
	eol := "\n"
	if strings.Contains(s, "\r\n") {
		eol = "\r\n"
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	return lines, eol
}

func parseMapping(line string) (string, []string, bool) {
	l := line
	if i := strings.Index(l, "#"); i >= 0 {
		l = l[:i]
	}
	f := strings.Fields(l)
	if len(f) < 2 || net.ParseIP(f[0]) == nil {
		return "", nil, false
	}
	return f[0], f[1:], true
}

func markerOf(line string) (string, bool, bool) {
	m := markerRe.FindStringSubmatch(line)
	if m == nil {
		return "", false, false
	}
	isClose := closeRe.MatchString(line)
	if !isClose && !openRe.MatchString(line) {
		return "", false, false
	}
	return strings.ToUpper(m[1]), true, isClose
}

func Parse(content []byte) Parsed {
	lines, _ := splitLines(content)
	p := Parsed{Sections: []string{}, RiotEntries: []Entry{}, Pinned: map[string]string{}}
	current := ""
	seen := map[string]bool{}
	for i, line := range lines {
		if name, ok, isClose := markerOf(line); ok {
			if isClose {
				current = ""
			} else {
				current = name
				if !seen[name] {
					seen[name] = true
					p.Sections = append(p.Sections, name)
				}
			}
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		ip, hosts, ok := parseMapping(line)
		if !ok {
			continue
		}
		for _, h := range hosts {
			h = strings.ToLower(h)
			if probe.IsRiotHost(h) {
				p.RiotEntries = append(p.RiotEntries, Entry{IP: ip, Host: h, Line: i + 1, Section: current})
				if net.ParseIP(ip).To4() != nil {
					p.Pinned[h] = ip
				}
			}
		}
	}
	return p
}

func Clean(content []byte) ([]byte, []string, bool) {
	lines, eol := splitLines(content)
	var kept, removed []string
	inSection := false
	for _, line := range lines {
		if _, ok, isClose := markerOf(line); ok {
			removed = append(removed, line)
			inSection = !isClose
			continue
		}
		if inSection {
			removed = append(removed, line)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			if _, hosts, ok := parseMapping(line); ok && anyRiot(hosts) {
				removed = append(removed, line)
				continue
			}
		}
		kept = append(kept, line)
	}
	if len(removed) == 0 {
		return content, nil, false
	}
	for len(kept) > 1 && strings.TrimSpace(kept[len(kept)-1]) == "" && strings.TrimSpace(kept[len(kept)-2]) == "" {
		kept = kept[:len(kept)-1]
	}
	return []byte(strings.Join(kept, eol)), removed, true
}

func Restore(current []byte, removed []string) []byte {
	lines, eol := splitLines(current)
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	lines = append(lines, "")
	lines = append(lines, removed...)
	lines = append(lines, "")
	return []byte(strings.Join(lines, eol))
}

func anyRiot(hosts []string) bool {
	for _, h := range hosts {
		if probe.IsRiotHost(h) {
			return true
		}
	}
	return false
}

func isManagedMarker(line string) (bool, bool) {
	name, ok, isClose := markerOf(line)
	return ok && name == ManagedMarker, isClose
}

func ApplyRoutes(content []byte, routes map[string]string, date string) ([]byte, []string) {
	lines, eol := splitLines(content)
	var kept, removed []string
	inManaged := false
	for _, line := range lines {
		if m, isClose := isManagedMarker(line); m {
			removed = append(removed, line)
			inManaged = !isClose
			continue
		}
		if inManaged {
			removed = append(removed, line)
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			if _, hosts, ok := parseMapping(line); ok && anyIn(hosts, routes) {
				removed = append(removed, line)
				continue
			}
		}
		kept = append(kept, line)
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	if len(routes) > 0 {
		kept = append(kept, "", "# >>> NETWICK >>> (managed routes, added "+date+" — safe to delete)")
		for _, h := range sortedHosts(routes) {
			kept = append(kept, padRight(routes[h], 18)+" "+h)
		}
		kept = append(kept, "# <<< NETWICK <<<")
	}
	kept = append(kept, "")
	return []byte(strings.Join(kept, eol)), removed
}

func RemoveManaged(content []byte) []byte {
	lines, eol := splitLines(content)
	var kept []string
	inManaged := false
	for _, line := range lines {
		if m, isClose := isManagedMarker(line); m {
			inManaged = !isClose
			continue
		}
		if inManaged {
			continue
		}
		kept = append(kept, line)
	}
	return []byte(strings.Join(kept, eol))
}

func anyIn(hosts []string, routes map[string]string) bool {
	for _, h := range hosts {
		if _, ok := routes[strings.ToLower(h)]; ok {
			return true
		}
	}
	return false
}

func sortedHosts(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}
