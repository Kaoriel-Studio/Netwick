//go:build windows

package winsys

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/kaoriel-studio/netwick/internal/platform"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modIphlpapi         = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetDNSSettings  = modIphlpapi.NewProc("GetInterfaceDnsSettings")
	procSetDNSSettings  = modIphlpapi.NewProc("SetInterfaceDnsSettings")
	procFreeDNSSettings = modIphlpapi.NewProc("FreeInterfaceDnsSettings")
	procLuidToGuid      = modIphlpapi.NewProc("ConvertInterfaceLuidToGuid")
	modSrclient         = windows.NewLazySystemDLL("srclient.dll")
	procSRSetRestorePnt = modSrclient.NewProc("SRSetRestorePointW")
)

type Actions struct {
	sys *System
}

func NewActions(sys *System) *Actions { return &Actions{sys: sys} }

func runSystemTool(ctx context.Context, exe string, args ...string) error {
	if strings.ContainsAny(exe, `\/`) {
		return platform.ErrDenied
	}
	cmd := exec.CommandContext(ctx, filepath.Join(system32(), exe), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	cmd.Dir = system32()
	return cmd.Run()
}

func (a *Actions) TerminateProcesses(names []string) (int, error) {
	kill := map[string]bool{}
	for _, n := range names {
		kill[strings.ToLower(n)+".exe"] = true
	}
	procs, err := a.sys.Processes()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range procs {
		if !kill[strings.ToLower(p.Name)] {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, p.PID)
		if err != nil {
			continue
		}
		if windows.TerminateProcess(h, 0) == nil {
			n++
			_, _ = windows.WaitForSingleObject(h, 3000)
		}
		windows.CloseHandle(h)
	}
	return n, nil
}

func (a *Actions) StartProcess(path string, args []string) error {
	if !filepath.IsAbs(path) {
		return platform.ErrDenied
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func (a *Actions) FlushDNS(ctx context.Context) error {
	return runSystemTool(ctx, "ipconfig.exe", "/flushdns")
}

func (a *Actions) RenewIP(ctx context.Context) error {
	if err := runSystemTool(ctx, "ipconfig.exe", "/release"); err != nil {
		return err
	}
	return runSystemTool(ctx, "ipconfig.exe", "/renew")
}

func (a *Actions) ResetNetworkStack(ctx context.Context) error {
	if err := runSystemTool(ctx, "netsh.exe", "int", "ip", "reset"); err != nil {
		return err
	}
	return runSystemTool(ctx, "netsh.exe", "winsock", "reset")
}

type dnsInterfaceSettings struct {
	Version             uint32
	Flags               uint64
	Domain              *uint16
	NameServer          *uint16
	SearchList          *uint16
	RegistrationEnabled uint32
	RegisterAdapterName uint32
	EnableLLMNR         uint32
	QueryAdapterName    uint32
	ProfileNameServer   *uint16
}

const (
	dnsSettingsVersion1  = 1
	dnsSettingNameServer = 0x0002
)

func luidToGUID(luid uint64) (windows.GUID, error) {
	var g windows.GUID
	if err := procLuidToGuid.Find(); err != nil {
		return g, err
	}
	r, _, _ := procLuidToGuid.Call(uintptr(unsafe.Pointer(&luid)), uintptr(unsafe.Pointer(&g)))
	if r != 0 {
		return g, syscall.Errno(r)
	}
	return g, nil
}

func splitServers(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
	out := []string{}
	for _, x := range f {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func (a *Actions) GetDNS(luid uint64) (platform.DNSState, error) {
	g, err := luidToGUID(luid)
	if err != nil {
		return platform.DNSState{}, err
	}
	if err := procGetDNSSettings.Find(); err != nil {
		return platform.DNSState{}, platform.ErrUnavailable
	}
	s := dnsInterfaceSettings{Version: dnsSettingsVersion1}
	r, _, _ := procGetDNSSettings.Call(uintptr(unsafe.Pointer(&g)), uintptr(unsafe.Pointer(&s)))
	if r != 0 {
		return platform.DNSState{}, syscall.Errno(r)
	}
	defer procFreeDNSSettings.Call(uintptr(unsafe.Pointer(&s)))
	servers := splitServers(windows.UTF16PtrToString(s.NameServer))
	return platform.DNSState{LUID: luid, DHCP: len(servers) == 0, Servers: servers}, nil
}

func (a *Actions) SetDNS(luid uint64, dhcp bool, servers []string) error {
	g, err := luidToGUID(luid)
	if err != nil {
		return err
	}
	if err := procSetDNSSettings.Find(); err != nil {
		return platform.ErrUnavailable
	}
	value := ""
	if !dhcp {
		value = strings.Join(servers, ",")
	}
	ns, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return err
	}
	s := dnsInterfaceSettings{Version: dnsSettingsVersion1, Flags: dnsSettingNameServer, NameServer: ns}
	r, _, _ := procSetDNSSettings.Call(uintptr(unsafe.Pointer(&g)), uintptr(unsafe.Pointer(&s)))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func startTypeValue(st platform.StartType) (uint32, bool) {
	switch st {
	case platform.StartAutomatic:
		return windows.SERVICE_AUTO_START, true
	case platform.StartManual:
		return windows.SERVICE_DEMAND_START, true
	case platform.StartDisabled:
		return windows.SERVICE_DISABLED, true
	case platform.StartSystem:
		return windows.SERVICE_SYSTEM_START, true
	case platform.StartBoot:
		return windows.SERVICE_BOOT_START, true
	}
	return 0, false
}

func openServiceFor(name string, access uint32) (windows.Handle, windows.Handle, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return 0, 0, err
	}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		windows.CloseServiceHandle(scm)
		return 0, 0, err
	}
	h, err := windows.OpenService(scm, p, access)
	if err != nil {
		windows.CloseServiceHandle(scm)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return 0, 0, platform.ErrNotFound
		}
		return 0, 0, err
	}
	return scm, h, nil
}

func (a *Actions) SetServiceStartType(name string, st platform.StartType) error {
	v, ok := startTypeValue(st)
	if !ok {
		return platform.ErrDenied
	}
	scm, h, err := openServiceFor(name, windows.SERVICE_CHANGE_CONFIG|windows.SERVICE_QUERY_CONFIG)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(scm)
	defer windows.CloseServiceHandle(h)
	return windows.ChangeServiceConfig(h, windows.SERVICE_NO_CHANGE, v, windows.SERVICE_NO_CHANGE, nil, nil, nil, nil, nil, nil, nil)
}

func (a *Actions) StopService(ctx context.Context, name string) error {
	scm, h, err := openServiceFor(name, windows.SERVICE_STOP|windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(scm)
	defer windows.CloseServiceHandle(h)
	var st windows.SERVICE_STATUS
	if err := windows.ControlService(h, windows.SERVICE_CONTROL_STOP, &st); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if err := windows.QueryServiceStatus(h, &st); err != nil {
			return err
		}
		if st.CurrentState == windows.SERVICE_STOPPED {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return errors.New("service did not stop")
}

func (a *Actions) StartService(name string) error {
	scm, h, err := openServiceFor(name, windows.SERVICE_START)
	if err != nil {
		return err
	}
	defer windows.CloseServiceHandle(scm)
	defer windows.CloseServiceHandle(h)
	err = windows.StartService(h, 0, nil)
	if errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return nil
	}
	return err
}

const crashControlKey = `SYSTEM\CurrentControlSet\Control\CrashControl`

func (a *Actions) GetCrashDumpEnabled() (platform.RegDWORD, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, crashControlKey, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return platform.RegDWORD{}, err
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("CrashDumpEnabled")
	if errors.Is(err, registry.ErrNotExist) {
		return platform.RegDWORD{}, nil
	}
	if err != nil {
		return platform.RegDWORD{}, err
	}
	return platform.RegDWORD{Exists: true, Value: uint32(v)}, nil
}

func (a *Actions) SetCrashDumpEnabled(v platform.RegDWORD) error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, crashControlKey, registry.SET_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return err
	}
	defer k.Close()
	if !v.Exists {
		err := k.DeleteValue("CrashDumpEnabled")
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	return k.SetDWordValue("CrashDumpEnabled", v.Value)
}

func (a *Actions) WriteHosts(content []byte) error {
	target := a.sys.HostsPath()
	tmp := target + ".netwick-tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	from, _ := windows.UTF16PtrFromString(tmp)
	to, _ := windows.UTF16PtrFromString(target)
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

const (
	beginSystemChange = 100
	endSystemChange   = 101
	modifySettings    = 12
)

func srCall(event, kind uint32, seq int64, desc string) (bool, uint32, int64) {
	var info [528]byte
	binary.LittleEndian.PutUint32(info[0:], event)
	binary.LittleEndian.PutUint32(info[4:], kind)
	binary.LittleEndian.PutUint64(info[8:], uint64(seq))
	u, _ := windows.UTF16FromString(desc)
	if len(u) > 255 {
		u = append(u[:255], 0)
	}
	for i, c := range u {
		binary.LittleEndian.PutUint16(info[16+2*i:], c)
	}
	var status [12]byte
	r, _, _ := procSRSetRestorePnt.Call(uintptr(unsafe.Pointer(&info[0])), uintptr(unsafe.Pointer(&status[0])))
	return r != 0, binary.LittleEndian.Uint32(status[0:]), int64(binary.LittleEndian.Uint64(status[4:]))
}

func latestRestorePoint() int64 {
	rows, err := wmiQuery(`root\default`, "SELECT SequenceNumber FROM SystemRestore", []string{"SequenceNumber"})
	if err != nil {
		return -1
	}
	var maxSeq int64 = 0
	for _, r := range rows {
		if n, ok := asInt(r["SequenceNumber"]); ok && n > maxSeq {
			maxSeq = n
		}
	}
	return maxSeq
}

func (a *Actions) CreateRestorePoint(description string) error {
	if err := procSRSetRestorePnt.Find(); err != nil {
		return platform.ErrUnavailable
	}
	before := latestRestorePoint()
	var result error
	err := withCOM(func() error {
		ok, status, seq := srCall(beginSystemChange, modifySettings, 0, description)
		if !ok {
			if status == uint32(windows.ERROR_SERVICE_DISABLED) {
				result = platform.ErrUnavailable
			} else {
				result = syscall.Errno(status)
			}
			return nil
		}
		srCall(endSystemChange, modifySettings, seq, description)
		return nil
	})
	if err != nil {
		return err
	}
	if result != nil {
		return result
	}
	if after := latestRestorePoint(); before >= 0 && after >= 0 && after <= before {
		return platform.ErrUnavailable
	}
	return nil
}

func (a *Actions) BackupDir() string {
	pd, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil || pd == "" {
		pd = `C:\ProgramData`
	}
	dir := filepath.Join(pd, "Netwick", "backups")
	_ = SecureDir(dir, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
	return dir
}

func SecureDir(dir, sddl string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

func CurrentUserSID() (string, error) {
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}
