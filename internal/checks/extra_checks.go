package checks

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/riotstatus"
)

var (
	regionMu sync.Mutex
	region   = riotstatus.DefaultRegion
)

func SetRegion(r string) {
	if !riotstatus.ValidRegion(r) {
		r = riotstatus.DefaultRegion
	}
	regionMu.Lock()
	region = r
	regionMu.Unlock()
}

func Region() string {
	regionMu.Lock()
	defer regionMu.Unlock()
	return region
}

type RiotStatusCheck struct{ base }

func (c *RiotStatusCheck) Timeout() time.Duration { return 7 * time.Second }

func (c *RiotStatusCheck) Run(ctx context.Context, sys platform.System) core.Result {
	reg := Region()
	type fetched struct {
		st riotstatus.Status
		ok bool
	}
	out := make([]fetched, len(riotstatus.Products))
	var wg sync.WaitGroup
	for i, p := range riotstatus.Products {
		wg.Add(1)
		go func(i int, p riotstatus.Product) {
			defer wg.Done()
			b, err := sys.Network().FetchStatus(ctx, riotstatus.URL(p, reg))
			if err != nil {
				return
			}
			st, err := riotstatus.Parse(p.ID, b)
			if err != nil {
				return
			}
			out[i] = fetched{st: st, ok: true}
		}(i, p)
	}
	wg.Wait()
	r := c.result(core.StatusOK, "riotstatus.ok")
	r.SourceURL = core.SrcServerStatus
	r.Params["region"] = reg
	reached, incidents, maint := 0, 0, 0
	for _, f := range out {
		if !f.ok {
			continue
		}
		reached++
		for _, it := range f.st.Incidents {
			incidents++
			r.AddDetail(core.StatusWarn, "riotstatus.incident_item", map[string]string{"product": f.st.Product, "title_en": it.TitleEN, "title_ru": it.TitleRU})
			if incidents == 1 {
				r.Params["product"] = f.st.Product
				r.Params["title_en"] = it.TitleEN
				r.Params["title_ru"] = it.TitleRU
			}
		}
		for _, it := range f.st.Maintenances {
			maint++
			r.AddDetail(core.StatusInfo, "riotstatus.maintenance_item", map[string]string{"product": f.st.Product, "title_en": it.TitleEN, "title_ru": it.TitleRU})
			if incidents == 0 && maint == 1 {
				r.Params["product"] = f.st.Product
				r.Params["title_en"] = it.TitleEN
				r.Params["title_ru"] = it.TitleRU
			}
		}
	}
	r.Params["count"] = strconv.Itoa(incidents + maint)
	switch {
	case reached == 0:
		r.Status = core.StatusUnknown
		r.MessageKey = "riotstatus.unreachable"
	case incidents > 0:
		r.Status = core.StatusWarn
		r.MessageKey = "riotstatus.incident"
		r.Steps = []core.Step{{Kind: core.StepInfo, ID: "server_status"}, {Kind: core.StepInfo, ID: "wait"}}
	case maint > 0:
		r.Status = core.StatusWarn
		r.MessageKey = "riotstatus.maintenance"
		r.Steps = []core.Step{{Kind: core.StepInfo, ID: "server_status"}, {Kind: core.StepInfo, ID: "wait"}}
	}
	return r
}

const BonjourService = "Bonjour Service"

type BonjourCheck struct{ base }

func (c *BonjourCheck) Run(ctx context.Context, sys platform.System) core.Result {
	s, err := sys.Service(BonjourService)
	var r core.Result
	switch {
	case err == nil && s.Exists && s.State == platform.StateRunning:
		r = c.result(core.StatusInfo, "bonjour.running")
	case err == nil && s.Exists:
		r = c.result(core.StatusOK, "bonjour.stopped")
	default:
		r = c.result(core.StatusOK, "bonjour.none")
	}
	r.Data = s
	return r
}

type PowerCheck struct{ base }

func (c *PowerCheck) Run(ctx context.Context, sys platform.System) core.Result {
	p, err := sys.Power()
	if err != nil {
		return c.unknown(err)
	}
	var issues []string
	if p.Plan == platform.PlanPowerSaver {
		issues = append(issues, "saver")
	}
	if p.OnBattery == platform.Yes {
		issues = append(issues, "battery")
	}
	if p.GameMode == platform.No {
		issues = append(issues, "gamemode")
	}
	var r core.Result
	if len(issues) == 0 {
		r = c.result(core.StatusOK, "power.ok")
	} else {
		r = c.result(core.StatusInfo, "power.issues")
		r.GuideIDs = []string{"G15"}
		for _, i := range issues {
			r.AddDetail(core.StatusInfo, "power.issue."+i, nil)
		}
	}
	r.Params["plan"] = string(p.Plan)
	r.Data = p
	return r
}
