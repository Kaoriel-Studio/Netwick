package platform

import (
	"context"
	"errors"
	"time"

	"github.com/kaoriel-studio/netwick/internal/probe"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("unavailable")
	ErrDenied      = errors.New("access denied")
)

type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

type OSVersion struct {
	Major          uint32
	Minor          uint32
	Build          uint32
	UBR            uint32
	ProductName    string
	DisplayVersion string
	IsServer       bool
}

func (v OSVersion) IsWindows11() bool { return v.Major == 10 && v.Build >= 22000 }

type TriState int

const (
	Unknown TriState = iota
	Yes
	No
)

type TPMInfo struct {
	Present TriState
	Version string
}

type DeviceGuardInfo struct {
	VBS           TriState
	HVCI          TriState
	DMAProtection TriState
}

type StartType string

const (
	StartBoot      StartType = "boot"
	StartSystem    StartType = "system"
	StartAutomatic StartType = "automatic"
	StartManual    StartType = "manual"
	StartDisabled  StartType = "disabled"
	StartUnknown   StartType = "unknown"
)

func ValidStartType(s StartType) bool {
	switch s {
	case StartBoot, StartSystem, StartAutomatic, StartManual, StartDisabled:
		return true
	}
	return false
}

type ServiceState string

const (
	StateRunning ServiceState = "running"
	StateStopped ServiceState = "stopped"
	StatePending ServiceState = "pending"
	StateUnknown ServiceState = "unknown"
)

type ServiceInfo struct {
	Name        string
	DisplayName string
	Exists      bool
	State       ServiceState
	StartType   StartType
}

type ProxyInfo struct {
	Enabled   bool
	Server    string
	AutoURL   bool
	AutoProxy bool
}

const (
	IfTypeEthernet = 6
	IfTypeWiFi     = 71
	IfTypePPP      = 23
	IfTypeTunnel   = 131
)

type Adapter struct {
	LUID        uint64
	Name        string
	Description string
	IfType      uint32
	Up          bool
	DNS         []string
	HasGateway  bool
}

type FirewallRule struct {
	Name        string
	Application string
	Enabled     bool
	Block       bool
	Inbound     bool
}

type Process struct {
	PID  uint32
	Name string
}

type DriverFile struct {
	Name    string
	Exists  bool
	Version string
}

type Hardware struct {
	NativeArch   string
	SSE42        bool
	AVX          bool
	RAMBytes     uint64
	Manufacturer string
	Model        string
}

type PendingReboot struct {
	WindowsUpdate bool
	CBS           bool
	FileRename    bool
}

type PowerPlan string

const (
	PlanBalanced    PowerPlan = "balanced"
	PlanHigh        PowerPlan = "high"
	PlanUltimate    PowerPlan = "ultimate"
	PlanPowerSaver  PowerPlan = "saver"
	PlanUnknownPlan PowerPlan = "unknown"
)

type PowerInfo struct {
	Plan      PowerPlan
	OnBattery TriState
	GameMode  TriState
}

type System interface {
	Name() string
	Env(name string) string
	ReadFile(path string, max int64) ([]byte, error)
	ReadFileTail(path string, max int64) ([]byte, error)
	Stat(path string) (FileInfo, error)
	FindFiles(dir, pattern string, maxDepth int) ([]FileInfo, error)
	FixedDrives() []string
	OSVersion() (OSVersion, error)
	TPM() (TPMInfo, error)
	SecureBoot() (TriState, error)
	DeviceGuard() (DeviceGuardInfo, error)
	Service(name string) (ServiceInfo, error)
	ServiceByDisplayName(display string) (ServiceInfo, error)
	Proxy() (ProxyInfo, error)
	Adapters() ([]Adapter, error)
	FirewallRules() ([]FirewallRule, error)
	FirewallEnabled() (TriState, error)
	Processes() ([]Process, error)
	DriverFile(name string) (DriverFile, error)
	Hardware() (Hardware, error)
	PendingReboot() (PendingReboot, error)
	Power() (PowerInfo, error)
	DiskFree(path string) (uint64, error)
	HostsPath() string
	Now() time.Time
	Network() probe.Network
}

type DNSState struct {
	LUID    uint64   `json:"luid"`
	DHCP    bool     `json:"dhcp"`
	Servers []string `json:"servers"`
}

type FirewallAllow struct {
	Name        string
	Application string
}

type ServicePrev struct {
	Name       string    `json:"name"`
	StartType  StartType `json:"startType"`
	WasRunning bool      `json:"wasRunning"`
}

type RegDWORD struct {
	Exists bool   `json:"exists"`
	Value  uint32 `json:"value"`
}

type Actions interface {
	TerminateProcesses(names []string) (int, error)
	StartProcess(path string, args []string) error
	FlushDNS(ctx context.Context) error
	RenewIP(ctx context.Context) error
	GetDNS(luid uint64) (DNSState, error)
	SetDNS(luid uint64, dhcp bool, servers []string) error
	ResetNetworkStack(ctx context.Context) error
	AddFirewallAllow(rules []FirewallAllow) error
	RemoveFirewallRules(names []string) error
	FirewallRuleExists(name string) (bool, error)
	SetServiceStartType(name string, st StartType) error
	StopService(ctx context.Context, name string) error
	StartService(name string) error
	GetCrashDumpEnabled() (RegDWORD, error)
	SetCrashDumpEnabled(v RegDWORD) error
	WriteHosts(content []byte) error
	BackupDir() string
	RemoveTree(path string) error
	RemoveFile(path string) error
	ScheduleReboot(ctx context.Context) error
	CancelReboot(ctx context.Context) error
}
