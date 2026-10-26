//go:build windows

package winsys

import (
	"errors"
	"strings"
	"testing"
	"unsafe"

	"github.com/kaoriel-studio/netwick/internal/platform"
)

func TestStructLayout(t *testing.T) {
	if s := unsafe.Sizeof(dnsInterfaceSettings{}); s != 64 {
		t.Errorf("DNS_INTERFACE_SETTINGS size %d", s)
	}
	if s := unsafe.Sizeof(tpmDeviceInfo{}); s != 16 {
		t.Errorf("TPM_DEVICE_INFO size %d", s)
	}
	if s := unsafe.Sizeof(memoryStatusEx{}); s != 64 {
		t.Errorf("MEMORYSTATUSEX size %d", s)
	}
}

func allowed(err error) bool {
	return err == nil || errors.Is(err, platform.ErrUnavailable) || errors.Is(err, platform.ErrNotFound) || errors.Is(err, platform.ErrDenied)
}

func TestReadOnlyIntegration(t *testing.T) {
	s := New("Netwick/test")
	v, err := s.OSVersion()
	if err != nil || v.Major < 10 || v.Build == 0 {
		t.Errorf("OSVersion %+v %v", v, err)
	}
	if len(s.FixedDrives()) == 0 {
		t.Error("no fixed drives")
	}
	if _, err := s.TPM(); !allowed(err) {
		t.Errorf("TPM: %v", err)
	}
	if _, err := s.SecureBoot(); !allowed(err) {
		t.Errorf("SecureBoot: %v", err)
	}
	if _, err := s.DeviceGuard(); !allowed(err) {
		t.Logf("DeviceGuard unavailable: %v", err)
	}
	if svc, err := s.Service("Dnscache"); err != nil || !svc.Exists {
		t.Errorf("Dnscache: %+v %v", svc, err)
	}
	if _, err := s.Service("netwick-does-not-exist"); !errors.Is(err, platform.ErrNotFound) {
		t.Errorf("missing service: %v", err)
	}
	if _, err := s.ServiceByDisplayName("Killer Network Service"); !allowed(err) {
		t.Errorf("Killer: %v", err)
	}
	if _, err := s.Proxy(); err != nil {
		t.Errorf("Proxy: %v", err)
	}
	ads, err := s.Adapters()
	if err != nil || len(ads) == 0 {
		t.Errorf("Adapters %d %v", len(ads), err)
	}
	if _, err := s.FirewallRules(); err != nil {
		t.Errorf("FirewallRules: %v", err)
	}
	if _, err := s.FirewallEnabled(); err != nil {
		t.Errorf("FirewallEnabled: %v", err)
	}
	procs, err := s.Processes()
	if err != nil || len(procs) < 5 {
		t.Errorf("Processes %d %v", len(procs), err)
	}
	if d, err := s.DriverFile("tcpip.sys"); err != nil || !d.Exists || d.Version == "" {
		t.Errorf("DriverFile %+v %v", d, err)
	}
	if _, err := s.DriverFile(`..\evil.sys`); err == nil {
		t.Error("path traversal accepted")
	}
	hw, err := s.Hardware()
	if err != nil || hw.RAMBytes == 0 {
		t.Errorf("Hardware %+v %v", hw, err)
	}
	if _, err := s.PendingReboot(); err != nil {
		t.Errorf("PendingReboot: %v", err)
	}
	if free, err := s.DiskFree(`C:\`); err != nil || free == 0 {
		t.Errorf("DiskFree %d %v", free, err)
	}
	if b, err := s.ReadFile(s.HostsPath(), 1<<20); err != nil || len(b) == 0 {
		t.Errorf("hosts: %v", err)
	}
	a := NewActions(s)
	if _, err := a.GetCrashDumpEnabled(); err != nil {
		t.Errorf("CrashDump read: %v", err)
	}
	for _, ad := range ads {
		if ad.Up && ad.HasGateway {
			if _, err := a.GetDNS(ad.LUID); err != nil && !strings.Contains(err.Error(), "not") {
				t.Logf("GetDNS %s: %v", ad.Name, err)
			}
			break
		}
	}
}
