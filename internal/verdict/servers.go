package verdict

import (
	"sort"
	"strconv"
	"strings"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/probe"
)

var NetworkPlan = []core.Step{
	core.Guide("G10"),
	core.Fix("F02"),
	core.Fix("F03"),
	core.Fix("F04"),
	core.Fix("F05"),
	core.Guide("G08"),
	core.Fix("F06"),
}

type ServerSummary struct {
	CoreDown   []string `json:"coreDown"`
	ExtraDown  []string `json:"extraDown"`
	Mixed      []string `json:"mixed"`
	LogErrors  []string `json:"logErrors"`
	PinnedDead []string `json:"pinnedDead"`
	CertIssue  []string `json:"certIssue"`
	AvgLatency int      `json:"avgLatency"`
}

func Summarize(reports []probe.HostReport, logHosts map[string]int) ServerSummary {
	s := ServerSummary{CoreDown: []string{}, ExtraDown: []string{}, Mixed: []string{}, LogErrors: []string{}, PinnedDead: []string{}, CertIssue: []string{}}
	sum, n := 0, 0
	for _, r := range reports {
		for _, p := range r.Probes {
			if p.OK() {
				sum += p.LatencyMs
				n++
			}
			if p.Pinned && !p.OK() {
				s.PinnedDead = append(s.PinnedDead, r.Host)
			}
			if p.Status == probe.StatusCertError && !contains(s.CertIssue, r.Host) {
				s.CertIssue = append(s.CertIssue, r.Host)
			}
		}
		switch {
		case r.Down() && r.Core:
			s.CoreDown = append(s.CoreDown, r.Host)
		case r.Down():
			s.ExtraDown = append(s.ExtraDown, r.Host)
		case r.Mixed():
			if logHosts[r.Host] > 0 {
				s.LogErrors = append(s.LogErrors, r.Host)
			} else {
				s.Mixed = append(s.Mixed, r.Host)
			}
		}
	}
	if n > 0 {
		s.AvgLatency = sum / n
	}
	for _, l := range [][]string{s.CoreDown, s.ExtraDown, s.Mixed, s.LogErrors, s.PinnedDead, s.CertIssue} {
		sort.Strings(l)
	}
	return s
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func Servers(id, group string, reports []probe.HostReport, logHosts map[string]int) core.Result {
	s := Summarize(reports, logHosts)
	coreTotal := 0
	for _, r := range reports {
		if r.Core {
			coreTotal++
		}
	}
	res := core.NewResult(id, group, core.StatusOK, "servers.ok")
	res.SourceURL = core.SrcConnection

	switch {
	case coreTotal > 0 && len(s.CoreDown) == coreTotal:
		res.Status = core.StatusFail
		res.MessageKey = "servers.all_down"
		res.Steps = append([]core.Step{}, NetworkPlan...)
	case len(s.CoreDown) > 0:
		res.Status = core.StatusFail
		res.MessageKey = "servers.some_down"
		res.Params["hosts"] = strings.Join(s.CoreDown, ", ")
		res.Params["count"] = strconv.Itoa(len(s.CoreDown))
		res.Steps = append([]core.Step{}, NetworkPlan...)
	}

	if len(s.PinnedDead) > 0 {
		res.AddDetail(core.StatusFail, "servers.pinned_dead", map[string]string{"hosts": strings.Join(s.PinnedDead, ", ")})
		if res.Status != core.StatusFail {
			res.Status = core.StatusFail
			res.MessageKey = "servers.pinned_dead"
			res.Params["hosts"] = strings.Join(s.PinnedDead, ", ")
		}
		res.Steps = append([]core.Step{core.Fix("F10")}, res.Steps...)
	}
	if len(s.LogErrors) > 0 {
		res.AddDetail(core.StatusWarn, "servers.client_errors", map[string]string{"hosts": strings.Join(s.LogErrors, ", ")})
		if res.Status == core.StatusOK {
			res.Status = core.StatusWarn
			res.MessageKey = "servers.client_errors"
			res.Params["hosts"] = strings.Join(s.LogErrors, ", ")
			res.Steps = core.AppendSteps(res.Steps, core.Fix("F01"), core.Fix("F02"))
		}
	}
	if len(s.Mixed) > 0 {
		res.AddDetail(core.StatusInfo, "servers.mixed", map[string]string{"hosts": strings.Join(s.Mixed, ", ")})
	}
	if len(s.ExtraDown) > 0 {
		res.AddDetail(core.StatusInfo, "servers.extra_down", map[string]string{"hosts": strings.Join(s.ExtraDown, ", ")})
	}
	if len(s.CertIssue) > 0 {
		res.AddDetail(core.StatusInfo, "servers.cert_issue", map[string]string{"hosts": strings.Join(s.CertIssue, ", ")})
	}
	for _, r := range reports {
		ok := r.OkCount()
		res.AddDetail(hostStatus(r), "servers.host", map[string]string{
			"host":  r.Host,
			"ok":    strconv.Itoa(ok),
			"total": strconv.Itoa(len(r.Probes)),
			"core":  strconv.FormatBool(r.Core),
		})
	}
	if s.AvgLatency > 0 {
		res.Params["latency"] = strconv.Itoa(s.AvgLatency)
	}
	res.Data = s
	return res.Normalized()
}

func hostStatus(r probe.HostReport) core.Status {
	switch {
	case r.Down() && r.Core:
		return core.StatusFail
	case r.Down(), r.Mixed():
		return core.StatusInfo
	default:
		return core.StatusOK
	}
}
