package checks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/platform"
)

type Check interface {
	ID() string
	Group() string
	Run(ctx context.Context, sys platform.System) core.Result
}

type Dependent interface {
	DependsOn() []string
	RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result
}

type Timed interface {
	Timeout() time.Duration
}

type ProgressEvent struct {
	CheckID string      `json:"checkId"`
	Group   string      `json:"group"`
	Status  core.Status `json:"status"`
	Done    int         `json:"done"`
	Total   int         `json:"total"`
}

type Runner struct {
	Checks      []Check
	Concurrency int
	Budget      time.Duration
	Default     time.Duration
	OnProgress  func(ProgressEvent)
}

func NewRunner(list []Check) *Runner {
	return &Runner{Checks: list, Concurrency: 8, Budget: 10 * time.Second, Default: 3 * time.Second}
}

func (r *Runner) Run(ctx context.Context, sys platform.System) map[string]core.Result {
	ctx, cancel := context.WithTimeout(ctx, r.Budget)
	defer cancel()

	byID := map[string]Check{}
	for _, c := range r.Checks {
		byID[c.ID()] = c
	}
	done := map[string]chan struct{}{}
	for _, c := range r.Checks {
		done[c.ID()] = make(chan struct{})
	}
	results := map[string]core.Result{}
	var mu sync.Mutex
	sem := make(chan struct{}, max(1, r.Concurrency))
	finished := 0
	total := len(r.Checks)

	var wg sync.WaitGroup
	for _, c := range r.Checks {
		wg.Add(1)
		go func(c Check) {
			defer wg.Done()
			defer close(done[c.ID()])
			deps := map[string]core.Result{}
			if d, ok := c.(Dependent); ok {
				for _, id := range d.DependsOn() {
					ch, ok := done[id]
					if !ok {
						continue
					}
					select {
					case <-ch:
					case <-ctx.Done():
					}
					mu.Lock()
					if res, ok := results[id]; ok {
						deps[id] = res
					}
					mu.Unlock()
				}
			}
			var res core.Result
			select {
			case sem <- struct{}{}:
				res = r.runOne(ctx, sys, c, deps)
				<-sem
			case <-ctx.Done():
				res = core.NewResult(c.ID(), c.Group(), core.StatusUnknown, "check.timeout")
			}
			mu.Lock()
			results[c.ID()] = res
			finished++
			ev := ProgressEvent{CheckID: c.ID(), Group: c.Group(), Status: res.Status, Done: finished, Total: total}
			mu.Unlock()
			if r.OnProgress != nil {
				r.OnProgress(ev)
			}
		}(c)
	}
	wg.Wait()
	return results
}

func (r *Runner) runOne(parent context.Context, sys platform.System, c Check, deps map[string]core.Result) (res core.Result) {
	timeout := r.Default
	if t, ok := c.(Timed); ok {
		timeout = t.Timeout()
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	start := time.Now()
	defer func() {
		if p := recover(); p != nil {
			res = core.NewResult(c.ID(), c.Group(), core.StatusUnknown, "check.crashed")
			res.Params["error"] = fmt.Sprint(p)
		}
		res.CheckID = c.ID()
		res.Group = c.Group()
		res.DurationMs = time.Since(start).Milliseconds()
		res = res.Normalized()
	}()

	type out struct{ r core.Result }
	ch := make(chan out, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				r := core.NewResult(c.ID(), c.Group(), core.StatusUnknown, "check.crashed")
				r.Params["error"] = fmt.Sprint(p)
				ch <- out{r}
			}
		}()
		if d, ok := c.(Dependent); ok {
			ch <- out{d.RunWith(ctx, sys, deps)}
			return
		}
		ch <- out{c.Run(ctx, sys)}
	}()
	select {
	case o := <-ch:
		return o.r
	case <-ctx.Done():
		return core.NewResult(c.ID(), c.Group(), core.StatusUnknown, "check.timeout")
	}
}

type base struct {
	id    string
	group string
}

func (b base) ID() string    { return b.id }
func (b base) Group() string { return b.group }

func (b base) result(status core.Status, key string) core.Result {
	return core.NewResult(b.id, b.group, status, key)
}

func (b base) unknown(err error) core.Result {
	r := b.result(core.StatusUnknown, "check.unknown")
	if err != nil {
		r.Params["error"] = err.Error()
	}
	return r
}

func All() []Check {
	return []Check{
		&InstallCheck{base{"C01", core.GroupInstall}},
		&WindowsVersionCheck{base{"C02", core.GroupWindows}},
		&TimeCheck{base{"C03", core.GroupWindows}},
		&TPMCheck{base{"C04", core.GroupWindows}},
		&SecureBootCheck{base{"C05", core.GroupWindows}},
		&VBSCheck{base{"C06", core.GroupWindows}},
		&IOMMUCheck{base{"C07", core.GroupWindows}},
		&OnDemandCheck{base{"C08", core.GroupVanguard}},
		&VanguardCheck{base{"C09", core.GroupVanguard}},
		&ServersCheck{base: base{"C10", core.GroupRiot}},
		&ProxyVPNCheck{base{"C11", core.GroupNetwork}},
		&FirewallCheck{base{"C12", core.GroupNetwork}},
		&ConnectionTypeCheck{base{"C13", core.GroupNetwork}},
		&NetworkHogsCheck{base{"C14", core.GroupNetwork}},
		&DriversCheck{base{"C15", core.GroupWindows}},
		&KillerCheck{base{"C16", core.GroupNetwork}},
		&SpecsCheck{base{"C17", core.GroupWindows}},
		&PendingRebootCheck{base{"C18", core.GroupWindows}},
		&HostsCheck{base{"C19", core.GroupNetwork}},
		&LogsCheck{base{"C20", core.GroupRiot}},
		&DiskCheck{base{"C21", core.GroupInstall}},
		&RiotStatusCheck{base{"C22", core.GroupRiot}},
		&BonjourCheck{base{"C23", core.GroupNetwork}},
		&PowerCheck{base{"C24", core.GroupWindows}},
	}
}

const LastID = "C24"

var QuickIDs = []string{"C03", "C09", "C10", "C11", "C13", "C18", "C22"}

func Quick() []Check {
	return WithDeps(QuickIDs)
}

func ByID(id string) Check {
	for _, c := range All() {
		if c.ID() == id {
			return c
		}
	}
	return nil
}

func WithDeps(ids []string) []Check {
	need := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if need[id] {
			return
		}
		need[id] = true
		if c := ByID(id); c != nil {
			if d, ok := c.(Dependent); ok {
				for _, dep := range d.DependsOn() {
					visit(dep)
				}
			}
		}
	}
	for _, id := range ids {
		visit(id)
	}
	var out []Check
	for _, c := range All() {
		if need[c.ID()] {
			out = append(out, c)
		}
	}
	return out
}
