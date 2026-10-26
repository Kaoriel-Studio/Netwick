//go:build windows

package winsys

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/probe"
	"golang.org/x/sys/cpu"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modTbs      = windows.NewLazySystemDLL("tbs.dll")
	procTbsInfo = modTbs.NewProc("Tbsi_GetDeviceInfo")

	modKernel32       = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemory  = modKernel32.NewProc("GlobalMemoryStatusEx")
	modAdvapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procServiceKeyNme = modAdvapi32.NewProc("GetServiceKeyNameW")
)

type System struct {
	net probe.Network
}

func New(userAgent string) *System {
	return &System{net: probe.NewRealNetwork(userAgent)}
}

func (s *System) Name() string { return "windows" }

func (s *System) Env(name string) string { return os.Getenv(name) }

func (s *System) ReadFile(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if max <= 0 {
		max = 16 << 20
	}
	return io.ReadAll(io.LimitReader(f, max))
}

func (s *System) ReadFileTail(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if max > 0 && fi.Size() > max {
		if _, err := f.Seek(fi.Size()-max, io.SeekStart); err != nil {
			return nil, err
		}
	}
	return io.ReadAll(io.LimitReader(f, max))
}

func (s *System) Stat(path string) (platform.FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return platform.FileInfo{}, err
	}
	return platform.FileInfo{Path: path, Size: fi.Size(), ModTime: fi.ModTime(), IsDir: fi.IsDir()}, nil
}

func (s *System) FindFiles(dir, pattern string, maxDepth int) ([]platform.FileInfo, error) {
	root := filepath.Clean(dir)
	base := strings.Count(root, string(os.PathSeparator))
	var out []platform.FileInfo
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if strings.Count(p, string(os.PathSeparator))-base >= maxDepth {
				return fs.SkipDir
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return fs.SkipDir
			}
			return nil
		}
		if ok, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(d.Name())); !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, platform.FileInfo{Path: p, Size: info.Size(), ModTime: info.ModTime()})
		if len(out) > 2000 {
			return fs.SkipAll
		}
		return nil
	})
	return out, err
}

func (s *System) FixedDrives() []string {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, _ := windows.UTF16PtrFromString(root)
		if windows.GetDriveType(p) == windows.DRIVE_FIXED {
			out = append(out, root)
		}
	}
	return out
}

func (s *System) OSVersion() (platform.OSVersion, error) {
	v := windows.RtlGetVersion()
	out := platform.OSVersion{Major: v.MajorVersion, Minor: v.MinorVersion, Build: v.BuildNumber, IsServer: v.ProductType != 1}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err == nil {
		defer k.Close()
		if ubr, _, err := k.GetIntegerValue("UBR"); err == nil {
			out.UBR = uint32(ubr)
		}
		out.ProductName, _, _ = k.GetStringValue("ProductName")
		out.DisplayVersion, _, _ = k.GetStringValue("DisplayVersion")
	}
	if out.IsWindows11() && strings.HasPrefix(out.ProductName, "Windows 10") {
		out.ProductName = "Windows 11" + strings.TrimPrefix(out.ProductName, "Windows 10")
	}
	if out.ProductName == "" {
		out.ProductName = "Windows"
	}
	return out, nil
}

type tpmDeviceInfo struct {
	StructVersion    uint32
	TPMVersion       uint32
	TPMInterfaceType uint32
	TPMImpRevision   uint32
}

const tbsTPMNotFound = 0x8028400F

func (s *System) TPM() (platform.TPMInfo, error) {
	if err := procTbsInfo.Find(); err != nil {
		return platform.TPMInfo{Present: platform.Unknown}, platform.ErrUnavailable
	}
	var info tpmDeviceInfo
	r, _, _ := procTbsInfo.Call(uintptr(unsafe.Sizeof(info)), uintptr(unsafe.Pointer(&info)))
	switch {
	case r == 0:
		switch info.TPMVersion {
		case 2:
			return platform.TPMInfo{Present: platform.Yes, Version: "2.0"}, nil
		case 1:
			return platform.TPMInfo{Present: platform.Yes, Version: "1.2"}, nil
		}
		return platform.TPMInfo{Present: platform.Yes}, nil
	case uint32(r) == tbsTPMNotFound:
		return platform.TPMInfo{Present: platform.No}, nil
	}
	return platform.TPMInfo{Present: platform.Unknown}, platform.ErrUnavailable
}

func (s *System) SecureBoot() (platform.TriState, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\SecureBoot\State`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if errors.Is(err, registry.ErrNotExist) {
		return platform.No, nil
	}
	if err != nil {
		return platform.Unknown, platform.ErrDenied
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("UEFISecureBootEnabled")
	if err != nil {
		return platform.No, nil
	}
	if v == 1 {
		return platform.Yes, nil
	}
	return platform.No, nil
}

func contains(list []any, want int64) bool {
	for _, v := range list {
		switch n := v.(type) {
		case int32:
			if int64(n) == want {
				return true
			}
		case uint32:
			if int64(n) == want {
				return true
			}
		case int64:
			if n == want {
				return true
			}
		case uint8:
			if int64(n) == want {
				return true
			}
		}
	}
	return false
}

func asInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int32:
		return int64(n), true
	case uint32:
		return int64(n), true
	case int64:
		return n, true
	case uint8:
		return int64(n), true
	case int16:
		return int64(n), true
	case uint16:
		return int64(n), true
	}
	return 0, false
}

func (s *System) DeviceGuard() (platform.DeviceGuardInfo, error) {
	out := platform.DeviceGuardInfo{VBS: platform.Unknown, HVCI: platform.Unknown, DMAProtection: platform.Unknown}
	rows, err := wmiQuery(`root\Microsoft\Windows\DeviceGuard`, "SELECT VirtualizationBasedSecurityStatus, SecurityServicesRunning, AvailableSecurityProperties FROM Win32_DeviceGuard", []string{"VirtualizationBasedSecurityStatus", "SecurityServicesRunning", "AvailableSecurityProperties"})
	if err != nil || len(rows) == 0 {
		return out, platform.ErrUnavailable
	}
	row := rows[0]
	if n, ok := asInt(row["VirtualizationBasedSecurityStatus"]); ok {
		if n == 2 {
			out.VBS = platform.Yes
		} else {
			out.VBS = platform.No
		}
	}
	if list, ok := row["SecurityServicesRunning"].([]any); ok {
		if contains(list, 2) {
			out.HVCI = platform.Yes
		} else {
			out.HVCI = platform.No
		}
	} else if out.VBS == platform.No {
		out.HVCI = platform.No
	}
	if list, ok := row["AvailableSecurityProperties"].([]any); ok {
		if contains(list, 3) {
			out.DMAProtection = platform.Yes
		} else {
			out.DMAProtection = platform.No
		}
	}
	return out, nil
}

func startTypeName(st uint32) platform.StartType {
	switch st {
	case windows.SERVICE_BOOT_START:
		return platform.StartBoot
	case windows.SERVICE_SYSTEM_START:
		return platform.StartSystem
	case windows.SERVICE_AUTO_START:
		return platform.StartAutomatic
	case windows.SERVICE_DEMAND_START:
		return platform.StartManual
	case windows.SERVICE_DISABLED:
		return platform.StartDisabled
	}
	return platform.StartUnknown
}

func stateName(st uint32) platform.ServiceState {
	switch st {
	case windows.SERVICE_RUNNING:
		return platform.StateRunning
	case windows.SERVICE_STOPPED:
		return platform.StateStopped
	case windows.SERVICE_START_PENDING, windows.SERVICE_STOP_PENDING, windows.SERVICE_CONTINUE_PENDING, windows.SERVICE_PAUSE_PENDING:
		return platform.StatePending
	}
	return platform.StateUnknown
}

func openSCM() (windows.Handle, error) {
	return windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
}

func queryService(scm windows.Handle, name string) (platform.ServiceInfo, error) {
	info := platform.ServiceInfo{Name: name, State: platform.StateUnknown, StartType: platform.StartUnknown}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return info, err
	}
	h, err := windows.OpenService(scm, p, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return info, platform.ErrNotFound
	}
	if err != nil {
		return info, err
	}
	defer windows.CloseServiceHandle(h)
	info.Exists = true
	var needed uint32
	_ = windows.QueryServiceConfig(h, nil, 0, &needed)
	if needed > 0 && needed < 64<<10 {
		buf := make([]byte, needed)
		cfg := (*windows.QUERY_SERVICE_CONFIG)(unsafe.Pointer(&buf[0]))
		if windows.QueryServiceConfig(h, cfg, needed, &needed) == nil {
			info.StartType = startTypeName(cfg.StartType)
			info.DisplayName = windows.UTF16PtrToString(cfg.DisplayName)
		}
	}
	var st windows.SERVICE_STATUS
	if windows.QueryServiceStatus(h, &st) == nil {
		info.State = stateName(st.CurrentState)
	}
	return info, nil
}

func (s *System) Service(name string) (platform.ServiceInfo, error) {
	scm, err := openSCM()
	if err != nil {
		return platform.ServiceInfo{Name: name}, err
	}
	defer windows.CloseServiceHandle(scm)
	return queryService(scm, name)
}

func serviceKeyName(scm windows.Handle, display string) (string, error) {
	d, err := windows.UTF16PtrFromString(display)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, 257)
	size := uint32(len(buf))
	r, _, e := procServiceKeyNme.Call(uintptr(scm), uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		if errors.Is(e, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return "", platform.ErrNotFound
		}
		return "", e
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func (s *System) ServiceByDisplayName(display string) (platform.ServiceInfo, error) {
	scm, err := openSCM()
	if err != nil {
		return platform.ServiceInfo{}, err
	}
	defer windows.CloseServiceHandle(scm)
	name, err := serviceKeyName(scm, display)
	if err != nil {
		return platform.ServiceInfo{}, platform.ErrNotFound
	}
	return queryService(scm, name)
}

func (s *System) Proxy() (platform.ProxyInfo, error) {
	var out platform.ProxyInfo
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return out, err
	}
	defer k.Close()
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil && v == 1 {
		out.Enabled = true
	}
	out.Server, _, _ = k.GetStringValue("ProxyServer")
	if u, _, err := k.GetStringValue("AutoConfigURL"); err == nil && u != "" {
		out.AutoURL = true
	}
	return out, nil
}

func (s *System) Adapters() ([]platform.Adapter, error) {
	size := uint32(16 << 10)
	var buf []byte
	for i := 0; i < 3; i++ {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_GATEWAYS|windows.GAA_FLAG_SKIP_MULTICAST|windows.GAA_FLAG_SKIP_ANYCAST, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return nil, err
		}
	}
	var out []platform.Adapter
	for a := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
		ad := platform.Adapter{
			LUID:        a.Luid,
			Name:        windows.UTF16PtrToString(a.FriendlyName),
			Description: windows.UTF16PtrToString(a.Description),
			IfType:      a.IfType,
			Up:          a.OperStatus == windows.IfOperStatusUp,
			HasGateway:  a.FirstGatewayAddress != nil,
			DNS:         []string{},
		}
		for d := a.FirstDnsServerAddress; d != nil; d = d.Next {
			if ip := d.Address.IP(); ip != nil && ip.To4() != nil {
				ad.DNS = append(ad.DNS, ip.String())
			}
		}
		out = append(out, ad)
	}
	return out, nil
}

func (s *System) Processes() ([]platform.Process, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []platform.Process
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		out = append(out, platform.Process{PID: e.ProcessID, Name: windows.UTF16ToString(e.ExeFile[:])})
	}
	return out, nil
}

func fileVersion(path string) string {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 || size > 1<<20 {
		return ""
	}
	buf := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buf[0])); err != nil {
		return ""
	}
	var fixed *windows.VS_FIXEDFILEINFO
	var l uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buf[0]), `\`, unsafe.Pointer(&fixed), &l); err != nil || fixed == nil {
		return ""
	}
	return itoa(fixed.FileVersionMS>>16) + "." + itoa(fixed.FileVersionMS&0xffff) + "." + itoa(fixed.FileVersionLS>>16) + "." + itoa(fixed.FileVersionLS&0xffff)
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func system32() string {
	if dir, err := windows.GetSystemDirectory(); err == nil && dir != "" {
		return dir
	}
	return `C:\Windows\System32`
}

func System32() string { return system32() }

func (s *System) DriverFile(name string) (platform.DriverFile, error) {
	if strings.ContainsAny(name, `\/:`) {
		return platform.DriverFile{}, platform.ErrDenied
	}
	p := filepath.Join(system32(), "drivers", name)
	if _, err := os.Stat(p); err != nil {
		return platform.DriverFile{Name: name}, nil
	}
	return platform.DriverFile{Name: name, Exists: true, Version: fileVersion(p)}, nil
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

const imageFileMachineARM64 = 0xAA64

func (s *System) Hardware() (platform.Hardware, error) {
	hw := platform.Hardware{NativeArch: "amd64", SSE42: cpu.X86.HasSSE42, AVX: cpu.X86.HasAVX}
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	if r, _, _ := procGlobalMemory.Call(uintptr(unsafe.Pointer(&ms))); r != 0 {
		hw.RAMBytes = ms.TotalPhys
	}
	var pm, nm uint16
	if windows.IsWow64Process2(windows.CurrentProcess(), &pm, &nm) == nil && nm == imageFileMachineARM64 {
		hw.NativeArch = "arm64"
	}
	if rows, err := wmiQuery(`root\cimv2`, "SELECT Manufacturer, Model FROM Win32_ComputerSystem", []string{"Manufacturer", "Model"}); err == nil && len(rows) > 0 {
		hw.Manufacturer, _ = rows[0]["Manufacturer"].(string)
		hw.Model, _ = rows[0]["Model"].(string)
	}
	return hw, nil
}

func keyExists(root registry.Key, path string) bool {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func (s *System) PendingReboot() (platform.PendingReboot, error) {
	var p platform.PendingReboot
	p.WindowsUpdate = keyExists(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`)
	p.CBS = keyExists(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`)
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager`, registry.QUERY_VALUE|registry.WOW64_64KEY); err == nil {
		if v, _, err := k.GetStringsValue("PendingFileRenameOperations"); err == nil && len(v) > 0 {
			p.FileRename = true
		}
		k.Close()
	}
	return p, nil
}

func (s *System) DiskFree(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, err
	}
	return free, nil
}

func (s *System) HostsPath() string {
	return filepath.Join(system32(), "drivers", "etc", "hosts")
}

func (s *System) Now() time.Time { return time.Now() }

func (s *System) Network() probe.Network { return s.net }
