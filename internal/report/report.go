package report

import (
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/verdict"
)

type Sanitizer struct {
	UserProfile string
	UserName    string
	Computer    string
	AllowIPs    map[string]bool
}

var (
	profileRe = regexp.MustCompile(`(?i)\b[a-z]:[\\/]+users[\\/]+[^\\/\s"'<>|:*?]+`)
	emailRe   = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	jwtRe     = regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{5,}\.[A-Za-z0-9_\-]{5,}(\.[A-Za-z0-9_\-]*)?`)
	uuidRe    = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	hexRe     = regexp.MustCompile(`(?i)\b[0-9a-f]{32,}\b`)
	b64Re     = regexp.MustCompile(`[A-Za-z0-9+/_\-]{40,}={0,2}`)
	ipv4Re    = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6Re    = regexp.MustCompile(`(?i)[0-9a-f:]*:[0-9a-f:]*:[0-9a-f:]+`)
	riotIDRe  = regexp.MustCompile(`[\p{L}\p{N}_.]{3,16}#[\p{L}\p{N}]{3,5}\b`)
	tokenKVRe = regexp.MustCompile(`(?i)(access_token|id_token|refresh_token|token|password|passwd|authorization|cookie|session)(["']?\s*[:=]\s*["']?)[^\s"'&,;]+`)
)

var publicDNS = map[string]bool{"1.1.1.1": true, "8.8.8.8": true, "8.8.4.4": true, "77.88.8.8": true, "127.0.0.1": true}

func (s Sanitizer) String(in string) string {
	out := in
	if s.UserProfile != "" {
		for _, v := range []string{s.UserProfile, strings.ReplaceAll(s.UserProfile, `\`, "/")} {
			out = replaceFold(out, v, "%USERPROFILE%")
		}
	}
	out = profileRe.ReplaceAllString(out, "%USERPROFILE%")
	out = tokenKVRe.ReplaceAllString(out, "$1$2<hidden>")
	out = jwtRe.ReplaceAllString(out, "<token>")
	out = emailRe.ReplaceAllString(out, "<email>")
	out = uuidRe.ReplaceAllString(out, "<id>")
	out = hexRe.ReplaceAllString(out, "<token>")
	out = b64Re.ReplaceAllStringFunc(out, func(m string) string {
		if strings.ContainsAny(m, "0123456789") {
			return "<token>"
		}
		return m
	})
	out = riotIDRe.ReplaceAllString(out, "<riot-id>")
	out = ipv4Re.ReplaceAllStringFunc(out, func(m string) string {
		if net.ParseIP(m) == nil || s.AllowIPs[m] || publicDNS[m] {
			return m
		}
		return "<ip>"
	})
	out = ipv6Re.ReplaceAllStringFunc(out, func(m string) string {
		if ip := net.ParseIP(m); ip != nil && ip.To4() == nil {
			return "<ip>"
		}
		return m
	})
	if len(s.Computer) >= 3 {
		out = replaceWord(out, s.Computer, "<pc>")
	}
	if len(s.UserName) >= 3 {
		out = replaceWord(out, s.UserName, "<user>")
	}
	return out
}

func replaceFold(s, old, repl string) string {
	if old == "" {
		return s
	}
	lo, lold := strings.ToLower(s), strings.ToLower(old)
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(lo[i:], lold)
		if j < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		b.WriteString(s[i : i+j])
		b.WriteString(repl)
		i += j + len(old)
	}
}

func replaceWord(s, word, repl string) string {
	re := regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}_])` + regexp.QuoteMeta(word) + `($|[^\p{L}\p{N}_])`)
	for {
		next := re.ReplaceAllString(s, "${1}"+repl+"${2}")
		if next == s {
			return s
		}
		s = next
	}
}

func (s Sanitizer) Map(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = s.String(v)
	}
	return out
}

func (s Sanitizer) Result(r core.Result) core.Result {
	r.Params = s.Map(r.Params)
	details := make([]core.Detail, len(r.Details))
	for i, d := range r.Details {
		d.Params = s.Map(d.Params)
		details[i] = d
	}
	r.Details = details
	r.Data = nil
	return r.Normalized()
}

type FixRecord struct {
	FixID  string    `json:"fixId"`
	At     time.Time `json:"at"`
	OK     bool      `json:"ok"`
	Undone bool      `json:"undone"`
}

type Report struct {
	App         string          `json:"app"`
	Version     string          `json:"version"`
	Mode        string          `json:"mode"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Verdict     verdict.Verdict `json:"verdict"`
	Results     []core.Result   `json:"results"`
	Fixes       []FixRecord     `json:"fixes"`
}

func AllowIPsFrom(results map[string]core.Result) map[string]bool {
	allow := map[string]bool{}
	if r, ok := results["C10"]; ok {
		if d, ok := r.Data.(interface{ ReportIPs() []string }); ok {
			for _, ip := range d.ReportIPs() {
				allow[ip] = true
			}
		}
	}
	return allow
}

func Build(app, version, mode string, now time.Time, results map[string]core.Result, v verdict.Verdict, fixes []FixRecord, s Sanitizer) Report {
	rep := Report{App: app, Version: version, Mode: mode, GeneratedAt: now.UTC(), Fixes: append([]FixRecord{}, fixes...)}
	ids := core.SortedKeys(results)
	for _, id := range ids {
		rep.Results = append(rep.Results, s.Result(results[id]))
	}
	if rep.Results == nil {
		rep.Results = []core.Result{}
	}
	sv := v
	sv.Params = s.Map(v.Params)
	sv.Problems = make([]core.Result, len(v.Problems))
	for i, p := range v.Problems {
		sv.Problems[i] = s.Result(p)
	}
	sv.Recommendations = make([]core.Result, len(v.Recommendations))
	for i, p := range v.Recommendations {
		sv.Recommendations[i] = s.Result(p)
	}
	rep.Verdict = sv
	sort.SliceStable(rep.Fixes, func(i, j int) bool { return rep.Fixes[i].At.Before(rep.Fixes[j].At) })
	return rep
}
