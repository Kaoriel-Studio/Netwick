//go:build windows

package winsys

import (
	"errors"
	"unsafe"

	"github.com/kaoriel-studio/netwick/internal/platform"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	modPowrprof       = windows.NewLazySystemDLL("powrprof.dll")
	procGetActiveSch  = modPowrprof.NewProc("PowerGetActiveScheme")
	procSysPowerState = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")
)

var planGUIDs = map[windows.GUID]platform.PowerPlan{
	{Data1: 0x381b4222, Data2: 0xf694, Data3: 0x41f0, Data4: [8]byte{0x96, 0x85, 0xff, 0x5b, 0xb2, 0x60, 0xdf, 0x2e}}: platform.PlanBalanced,
	{Data1: 0x8c5e7fda, Data2: 0xe8bf, Data3: 0x4a96, Data4: [8]byte{0x9a, 0x85, 0xa6, 0xe2, 0x3a, 0x8c, 0x63, 0x5c}}: platform.PlanHigh,
	{Data1: 0xa1841308, Data2: 0x3541, Data3: 0x4fab, Data4: [8]byte{0xbc, 0x81, 0xf7, 0x15, 0x56, 0xf2, 0x0b, 0x4a}}: platform.PlanPowerSaver,
	{Data1: 0xe9a42b02, Data2: 0xd5df, Data3: 0x448d, Data4: [8]byte{0xaa, 0x00, 0x03, 0xf1, 0x47, 0x49, 0xeb, 0x61}}: platform.PlanUltimate,
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

func (s *System) Power() (platform.PowerInfo, error) {
	out := platform.PowerInfo{Plan: platform.PlanUnknownPlan}
	if err := procGetActiveSch.Find(); err != nil {
		return out, platform.ErrUnavailable
	}
	var g *windows.GUID
	if r, _, _ := procGetActiveSch.Call(0, uintptr(unsafe.Pointer(&g))); r == 0 && g != nil {
		if p, ok := planGUIDs[*g]; ok {
			out.Plan = p
		}
		_, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(g)))
	}
	var st systemPowerStatus
	if r, _, _ := procSysPowerState.Call(uintptr(unsafe.Pointer(&st))); r != 0 {
		switch {
		case st.BatteryFlag == 128:
			out.OnBattery = platform.No
		case st.ACLineStatus == 0:
			out.OnBattery = platform.Yes
		case st.ACLineStatus == 1:
			out.OnBattery = platform.No
		}
	}
	out.GameMode = platform.Yes
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\GameBar`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()
		v, _, err := k.GetIntegerValue("AutoGameModeEnabled")
		switch {
		case err == nil && v == 0:
			out.GameMode = platform.No
		case err != nil && !errors.Is(err, registry.ErrNotExist):
			out.GameMode = platform.Unknown
		}
	}
	return out, nil
}
