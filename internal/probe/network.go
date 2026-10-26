package probe

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/riotstatus"
)

var CoreHosts = []string{
	"valorant.secure.dyn.riotcdn.net",
	"riot-client.secure.dyn.riotcdn.net",
	"clientconfig.rpg.riotgames.com",
	"auth.riotgames.com",
	"entitlements.auth.riotgames.com",
}

var PublicDNS = []string{"1.1.1.1", "8.8.8.8", "77.88.8.8"}

var RiotSuffixes = []string{"riotgames.com", "riotcdn.net", "pvp.net", "riotgames.co.kr"}

func IsRiotHost(h string) bool {
	h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	if h == "" || len(h) > 253 || strings.HasPrefix(h, ".") || strings.Contains(h, "..") {
		return false
	}
	for _, r := range h {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-') {
			return false
		}
	}
	for _, s := range RiotSuffixes {
		if h == s || strings.HasSuffix(h, "."+s) {
			return true
		}
	}
	return false
}

func IsCoreHost(h string) bool {
	for _, c := range CoreHosts {
		if c == h {
			return true
		}
	}
	return false
}

type Network interface {
	ResolveA(ctx context.Context, host, server string) ([]string, error)
	SystemResolveA(ctx context.Context, host string) ([]string, error)
	Probe(ctx context.Context, ip, host string) Result
	FetchStatus(ctx context.Context, url string) ([]byte, error)
	TCPPing(ctx context.Context, ip string) (time.Duration, error)
}

type RealNetwork struct {
	Config     Config
	DNSTimeout time.Duration
}

func NewRealNetwork(userAgent string) *RealNetwork {
	return &RealNetwork{Config: DefaultConfig(userAgent), DNSTimeout: 2500 * time.Millisecond}
}

func (n *RealNetwork) ResolveA(ctx context.Context, host, server string) ([]string, error) {
	if net.ParseIP(server) == nil {
		return nil, &net.DNSError{Err: "bad server", Name: host}
	}
	ctx, cancel := context.WithTimeout(ctx, n.DNSTimeout)
	defer cancel()
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: n.DNSTimeout}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}
	return lookup4(ctx, r, host)
}

func (n *RealNetwork) SystemResolveA(ctx context.Context, host string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, n.DNSTimeout)
	defer cancel()
	return lookup4(ctx, net.DefaultResolver, host)
}

func (n *RealNetwork) Probe(ctx context.Context, ip, host string) Result {
	return Probe(ctx, ip, host, n.Config)
}

func (n *RealNetwork) TCPPing(ctx context.Context, ip string) (time.Duration, error) {
	if net.ParseIP(ip).To4() == nil {
		return 0, errors.New("bad address")
	}
	d := net.Dialer{Timeout: 2 * time.Second}
	start := time.Now()
	c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, "443"))
	if err != nil {
		return 0, err
	}
	rtt := time.Since(start)
	_ = c.Close()
	return rtt, nil
}

var ErrStatusURL = errors.New("status URL not allowed")

func (n *RealNetwork) FetchStatus(ctx context.Context, url string) ([]byte, error) {
	if !riotstatus.ValidURL(url) {
		return nil, ErrStatusURL
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", n.Config.UserAgent)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, riotstatus.MaxBody))
}

func lookup4(ctx context.Context, r *net.Resolver, host string) ([]string, error) {
	ips, err := r.LookupIP(ctx, "ip4", host)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			out = append(out, v4.String())
		}
	}
	return out, nil
}

type HostReport struct {
	Host   string   `json:"host"`
	Core   bool     `json:"core"`
	Pool   []string `json:"pool"`
	Probes []Result `json:"probes"`
}

func (h HostReport) OkCount() int {
	n := 0
	for _, p := range h.Probes {
		if p.OK() {
			n++
		}
	}
	return n
}

func (h HostReport) Down() bool { return h.OkCount() == 0 }

func (h HostReport) Mixed() bool {
	ok := h.OkCount()
	return ok > 0 && ok < len(h.Probes)
}

type ProgressFunc func(done, total int)

func ResolvePool(ctx context.Context, nw Network, host string, servers []string) []string {
	var mu sync.Mutex
	seen := map[string]bool{}
	var order []string
	add := func(ips []string) {
		mu.Lock()
		defer mu.Unlock()
		for _, ip := range ips {
			if net.ParseIP(ip).To4() == nil || seen[ip] {
				continue
			}
			seen[ip] = true
			order = append(order, ip)
		}
	}
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			if ips, err := nw.ResolveA(ctx, host, s); err == nil {
				add(ips)
			}
		}(s)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if ips, err := nw.SystemResolveA(ctx, host); err == nil {
			add(ips)
		}
	}()
	wg.Wait()
	sort.Strings(order)
	return order
}

func ProbeHosts(ctx context.Context, nw Network, hosts []string, pinned map[string]string, limit int, progress ProgressFunc) []HostReport {
	if limit <= 0 {
		limit = 8
	}
	reports := make([]HostReport, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			pool := ResolvePool(ctx, nw, h, PublicDNS)
			if ip, ok := pinned[h]; ok && net.ParseIP(ip).To4() != nil {
				found := false
				for _, p := range pool {
					if p == ip {
						found = true
					}
				}
				if !found {
					pool = append(pool, ip)
				}
			}
			reports[i] = HostReport{Host: h, Core: IsCoreHost(h), Pool: pool, Probes: make([]Result, len(pool))}
		}(i, h)
	}
	wg.Wait()

	total := 0
	for _, r := range reports {
		total += len(r.Pool)
	}
	sem := make(chan struct{}, limit)
	var mu sync.Mutex
	done := 0
	for i := range reports {
		for j, ip := range reports[i].Pool {
			wg.Add(1)
			go func(i, j int, ip string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					reports[i].Probes[j] = Result{IP: ip, Host: reports[i].Host, Status: StatusNoTCP, Cause: CauseTimeout}
					return
				}
				res := nw.Probe(ctx, ip, reports[i].Host)
				<-sem
				res.IP, res.Host = ip, reports[i].Host
				if pinned[reports[i].Host] == ip {
					res.Pinned = true
				}
				reports[i].Probes[j] = res
				if progress != nil {
					mu.Lock()
					done++
					d := done
					mu.Unlock()
					progress(d, total)
				}
			}(i, j, ip)
		}
	}
	wg.Wait()
	return reports
}
