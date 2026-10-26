package fake

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/probe"
)

type file struct {
	data    []byte
	modTime time.Time
}

type ipState struct {
	status  probe.Status
	cause   probe.Cause
	latency int
}

type System struct {
	mu          sync.Mutex
	scenario    string
	env         map[string]string
	files       map[string]file
	drives      []string
	osv         platform.OSVersion
	tpm         platform.TPMInfo
	secureBoot  platform.TriState
	dg          platform.DeviceGuardInfo
	services    map[string]platform.ServiceInfo
	proxy       platform.ProxyInfo
	adapters    []platform.Adapter
	rules       []platform.FirewallRule
	fwEnabled   platform.TriState
	procs       []platform.Process
	drivers     map[string]platform.DriverFile
	hw          platform.Hardware
	pending     platform.PendingReboot
	diskFree    map[string]uint64
	clock       time.Duration
	dnsOK       bool
	pools       map[string][]string
	ips         map[string]ipState
	crashDump   platform.RegDWORD
	rpDisabled  bool
	dhcp        map[uint64]bool
	backupDir   string
	ActionDelay time.Duration
	ProbeDelay  time.Duration
}

const hostsPath = `C:\Windows\System32\drivers\etc\hosts`

func key(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, "/", `\`))
}

func (s *System) Name() string { return "demo:" + s.scenario }

func (s *System) Scenario() string { return s.scenario }

func (s *System) Env(name string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.env {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}

func (s *System) putFile(p string, data string, mod time.Time) {
	s.files[key(p)] = file{data: []byte(data), modTime: mod}
}

func (s *System) ReadFile(p string, max int64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[key(p)]
	if !ok {
		return nil, os.ErrNotExist
	}
	b := f.data
	if max > 0 && int64(len(b)) > max {
		b = b[:max]
	}
	return append([]byte{}, b...), nil
}

func (s *System) ReadFileTail(p string, max int64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[key(p)]
	if !ok {
		return nil, os.ErrNotExist
	}
	b := f.data
	if max > 0 && int64(len(b)) > max {
		b = b[int64(len(b))-max:]
	}
	return append([]byte{}, b...), nil
}

func (s *System) Stat(p string) (platform.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.files[key(p)]; ok {
		return platform.FileInfo{Path: p, Size: int64(len(f.data)), ModTime: f.modTime}, nil
	}
	prefix := key(p) + `\`
	for k := range s.files {
		if strings.HasPrefix(k, prefix) {
			return platform.FileInfo{Path: p, IsDir: true}, nil
		}
	}
	return platform.FileInfo{}, os.ErrNotExist
}

func (s *System) FindFiles(dir, pattern string, maxDepth int) ([]platform.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := key(dir) + `\`
	var out []platform.FileInfo
	for k, f := range s.files {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rel := strings.TrimPrefix(k, prefix)
		if strings.Count(rel, `\`) > maxDepth {
			continue
		}
		base := rel[strings.LastIndex(rel, `\`)+1:]
		if ok, _ := path.Match(strings.ToLower(pattern), base); ok {
			out = append(out, platform.FileInfo{Path: k, Size: int64(len(f.data)), ModTime: f.modTime})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (s *System) FixedDrives() []string { return append([]string{}, s.drives...) }

func (s *System) OSVersion() (platform.OSVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.osv, nil
}

func (s *System) TPM() (platform.TPMInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tpm, nil
}

func (s *System) SecureBoot() (platform.TriState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secureBoot, nil
}

func (s *System) DeviceGuard() (platform.DeviceGuardInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dg, nil
}

func (s *System) Service(name string) (platform.ServiceInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sv, ok := s.services[strings.ToLower(name)]; ok {
		return sv, nil
	}
	return platform.ServiceInfo{Name: name, State: platform.StateUnknown, StartType: platform.StartUnknown}, platform.ErrNotFound
}

func (s *System) ServiceByDisplayName(display string) (platform.ServiceInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sv := range s.services {
		if strings.EqualFold(sv.DisplayName, display) {
			return sv, nil
		}
	}
	return platform.ServiceInfo{}, platform.ErrNotFound
}

func (s *System) Proxy() (platform.ProxyInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.proxy, nil
}

func (s *System) Adapters() ([]platform.Adapter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]platform.Adapter, len(s.adapters))
	for i, a := range s.adapters {
		a.DNS = append([]string{}, a.DNS...)
		out[i] = a
	}
	return out, nil
}

func (s *System) FirewallRules() ([]platform.FirewallRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]platform.FirewallRule{}, s.rules...), nil
}

func (s *System) FirewallEnabled() (platform.TriState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fwEnabled, nil
}

func (s *System) Processes() ([]platform.Process, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]platform.Process{}, s.procs...), nil
}

func (s *System) DriverFile(name string) (platform.DriverFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.drivers[strings.ToLower(name)]; ok {
		return d, nil
	}
	return platform.DriverFile{Name: name}, nil
}

func (s *System) Hardware() (platform.Hardware, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hw, nil
}

func (s *System) PendingReboot() (platform.PendingReboot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pending, nil
}

func (s *System) DiskFree(p string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.diskFree[strings.ToUpper(p[:1])]; ok {
		return v, nil
	}
	return 0, platform.ErrNotFound
}

func (s *System) HostsPath() string { return hostsPath }

func (s *System) Now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Now().Add(s.clock)
}

func (s *System) Network() probe.Network { return (*fakeNet)(s) }

type fakeNet System

func (n *fakeNet) sys() *System { return (*System)(n) }

func (n *fakeNet) ResolveA(ctx context.Context, host, server string) ([]string, error) {
	s := n.sys()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dnsOK {
		return nil, context.DeadlineExceeded
	}
	return append([]string{}, s.pools[host]...), nil
}

func (n *fakeNet) SystemResolveA(ctx context.Context, host string) ([]string, error) {
	s := n.sys()
	b, _ := s.ReadFile(hostsPath, 0)
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && !strings.HasPrefix(f[0], "#") {
			for _, h := range f[1:] {
				if strings.EqualFold(h, host) {
					return []string{f[0]}, nil
				}
			}
		}
	}
	return n.ResolveA(ctx, host, "system")
}

func (n *fakeNet) Probe(ctx context.Context, ip, host string) probe.Result {
	s := n.sys()
	if s.ProbeDelay > 0 {
		select {
		case <-time.After(s.ProbeDelay):
		case <-ctx.Done():
			return probe.Result{IP: ip, Host: host, Status: probe.StatusNoTCP, Cause: probe.CauseTimeout}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.ips[ip]
	if !ok || !s.dnsOK {
		st = ipState{status: probe.StatusNoTCP, cause: probe.CauseTimeout}
	}
	r := probe.Result{IP: ip, Host: host, Status: st.status, Cause: st.cause, LatencyMs: st.latency}
	if st.status == probe.StatusOk || st.status == probe.StatusEmpty || st.status == probe.StatusCertError {
		now := time.Now().UTC()
		r.ServerDate = now.Truncate(time.Second)
		r.MeasuredAt = now.Add(s.clock)
		r.HTTPStatus = 403
		r.Bytes = 512
	}
	return r
}

func (s *System) TerminateProcesses(names []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kill := map[string]bool{}
	for _, n := range names {
		kill[strings.ToLower(n)+".exe"] = true
	}
	var kept []platform.Process
	n := 0
	for _, p := range s.procs {
		if kill[strings.ToLower(p.Name)] {
			n++
			continue
		}
		kept = append(kept, p)
	}
	s.procs = kept
	return n, nil
}

func (s *System) StartProcess(p string, args []string) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.procs = append(s.procs, platform.Process{PID: 4242, Name: filepath.Base(strings.ReplaceAll(p, `\`, "/"))})
	return nil
}

func (s *System) delay() {
	if s.ActionDelay > 0 {
		time.Sleep(s.ActionDelay)
	}
}

func (s *System) FlushDNS(ctx context.Context) error { s.delay(); return nil }

func (s *System) RenewIP(ctx context.Context) error { s.delay(); return nil }

func (s *System) GetDNS(luid uint64) (platform.DNSState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.adapters {
		if a.LUID == luid {
			st := platform.DNSState{LUID: luid, Servers: append([]string{}, a.DNS...)}
			st.DHCP = s.dhcp[luid]
			return st, nil
		}
	}
	return platform.DNSState{}, platform.ErrNotFound
}

func (s *System) SetDNS(luid uint64, dhcp bool, servers []string) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, a := range s.adapters {
		if a.LUID == luid {
			s.dhcp[luid] = dhcp
			if dhcp {
				s.adapters[i].DNS = []string{"192.168.1.1"}
			} else {
				s.adapters[i].DNS = append([]string{}, servers...)
				if s.scenario == "isp-cuts-some-ips" {
					s.healCut()
				}
			}
			return nil
		}
	}
	return platform.ErrNotFound
}

func (s *System) ResetNetworkStack(ctx context.Context) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending.CBS = true
	return nil
}

func (s *System) AddFirewallAllow(rules []platform.FirewallAllow) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range rules {
		s.rules = append(s.rules, platform.FirewallRule{Name: r.Name, Application: r.Application, Enabled: true, Inbound: true})
	}
	return nil
}

func (s *System) RemoveFirewallRules(names []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	del := map[string]bool{}
	for _, n := range names {
		del[n] = true
	}
	var kept []platform.FirewallRule
	for _, r := range s.rules {
		if !del[r.Name] {
			kept = append(kept, r)
		}
	}
	s.rules = kept
	return nil
}

func (s *System) FirewallRuleExists(name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rules {
		if r.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (s *System) SetServiceStartType(name string, st platform.StartType) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.services[strings.ToLower(name)]
	if !ok {
		return platform.ErrNotFound
	}
	sv.StartType = st
	s.services[strings.ToLower(name)] = sv
	return nil
}

func (s *System) StopService(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.services[strings.ToLower(name)]
	if !ok {
		return platform.ErrNotFound
	}
	sv.State = platform.StateStopped
	s.services[strings.ToLower(name)] = sv
	return nil
}

func (s *System) StartService(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.services[strings.ToLower(name)]
	if !ok {
		return platform.ErrNotFound
	}
	sv.State = platform.StateRunning
	s.services[strings.ToLower(name)] = sv
	return nil
}

func (s *System) GetCrashDumpEnabled() (platform.RegDWORD, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.crashDump, nil
}

func (s *System) SetCrashDumpEnabled(v platform.RegDWORD) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crashDump = v
	return nil
}

func (s *System) WriteHosts(content []byte) error {
	s.delay()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[key(hostsPath)] = file{data: append([]byte{}, content...), modTime: time.Now()}
	return nil
}

func (s *System) CreateRestorePoint(description string) error {
	s.delay()
	if s.rpDisabled {
		return platform.ErrUnavailable
	}
	return nil
}

func (s *System) BackupDir() string { return s.backupDir }

func (s *System) SimulateGuide(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch id {
	case "G14":
		var kept []platform.FirewallRule
		for _, r := range s.rules {
			if !r.Block {
				kept = append(kept, r)
			}
		}
		s.rules = kept
	case "G10":
		if !s.dnsOK {
			s.dnsOK = true
		}
	case "G08":
		s.proxy = platform.ProxyInfo{}
		var kept []platform.Adapter
		for _, a := range s.adapters {
			if !strings.Contains(strings.ToLower(a.Description), "wireguard") {
				kept = append(kept, a)
			}
		}
		s.adapters = kept
	case "G03":
		s.tpm = platform.TPMInfo{Present: platform.Yes, Version: "2.0"}
		s.secureBoot = platform.Yes
	case "G06", "reboot":
		s.pending = platform.PendingReboot{}
	case "time_sync":
		s.clock = 0
	}
}

func (s *System) healCut() {
	for ip, st := range s.ips {
		if st.status != probe.StatusOk {
			s.ips[ip] = ipState{status: probe.StatusOk, latency: 48}
		}
	}
}
