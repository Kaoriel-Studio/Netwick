package services

import (
	"context"
	"math"
	"time"

	"github.com/kaoriel-studio/netwick/internal/probe"
	"github.com/kaoriel-studio/netwick/internal/profile"
)

const qualitySamples = 12

var qualityInterval = 150 * time.Millisecond

type QualityView struct {
	Host    string `json:"host"`
	IP      string `json:"ip"`
	Samples []int  `json:"samples"`
	Min     int    `json:"min"`
	Avg     int    `json:"avg"`
	Max     int    `json:"max"`
	Jitter  int    `json:"jitter"`
	Loss    int    `json:"loss"`
}

func (t *ToolsService) QualityHosts() []string { return append([]string{}, probe.CoreHosts...) }

func (t *ToolsService) Quality(host string) (*QualityView, error) {
	if !probe.IsCoreHost(host) {
		return nil, ErrInvalid
	}
	if !t.st.qualMu.TryLock() {
		return nil, ErrBusy
	}
	defer t.st.qualMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	nw := t.st.Env.Sys.Network()
	v := &QualityView{Host: host, Samples: []int{}, Loss: 100}
	pool := probe.ResolvePool(ctx, nw, host, probe.PublicDNS)
	if len(pool) == 0 {
		return v, nil
	}
	v.IP = pool[0]
	lost, sum := 0, 0
	var ok []int
	for i := 0; i < qualitySamples; i++ {
		if i > 0 {
			select {
			case <-time.After(qualityInterval):
			case <-ctx.Done():
			}
		}
		if ctx.Err() != nil {
			break
		}
		d, err := nw.TCPPing(ctx, v.IP)
		if err != nil {
			lost++
			v.Samples = append(v.Samples, -1)
			continue
		}
		ms := int(math.Round(float64(d) / float64(time.Millisecond)))
		if ms < 1 {
			ms = 1
		}
		v.Samples = append(v.Samples, ms)
		ok = append(ok, ms)
		sum += ms
	}
	if len(v.Samples) > 0 {
		v.Loss = lost * 100 / len(v.Samples)
	}
	if len(ok) > 0 {
		v.Min, v.Max = ok[0], ok[0]
		for _, m := range ok {
			v.Min, v.Max = min(v.Min, m), max(v.Max, m)
		}
		v.Avg = sum / len(ok)
		if len(ok) > 1 {
			diff := 0
			for i := 1; i < len(ok); i++ {
				d := ok[i] - ok[i-1]
				if d < 0 {
					d = -d
				}
				diff += d
			}
			v.Jitter = diff / (len(ok) - 1)
		}
	}
	t.st.Prof.Bump(profile.CQuality)
	return v, nil
}
