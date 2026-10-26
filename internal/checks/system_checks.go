package checks

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/appcfg"
	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/platform"
)

const (
	MinBuild         = 19041
	RestrictionBuild = 19045
	Win11Build       = 22000
	Win11_25H2Build  = 26200
)

type InstallCheck struct{ base }

func (c *InstallCheck) Run(ctx context.Context, sys platform.System) core.Result {
	inst := ResolveInstall(sys)
	var r core.Result
	switch {
	case inst.ClientExe != "" && inst.ValorantExe != "":
		r = c.result(core.StatusOK, "install.ok")
	case inst.ClientExe != "":
		r = c.result(core.StatusWarn, "install.no_game")
		r.GuideIDs = []string{"G11"}
	default:
		r = c.result(core.StatusFail, "install.not_found")
		r.GuideIDs = []string{"G11"}
	}
	r.Params["client"] = inst.ClientExe
	r.Params["game"] = inst.ValorantExe
	r.Params["vgc"] = inst.VgcExe
	r.SourceURL = core.SrcMinSpecs
	r.Data = inst
	return r
}

type WindowsVersionCheck struct{ base }

func (c *WindowsVersionCheck) Run(ctx context.Context, sys platform.System) core.Result {
	v, err := sys.OSVersion()
	if err != nil {
		return c.unknown(err)
	}
	var r core.Result
	if v.Major < 10 || v.Build < MinBuild {
		r = c.result(core.StatusFail, "windows.unsupported")
		r.GuideIDs = []string{"G06"}
	} else {
		r = c.result(core.StatusOK, "windows.ok")
		if v.Build < RestrictionBuild {
			r.AddDetail(core.StatusInfo, "windows.restriction_hint", nil)
		}
	}
	r.Params["product"] = v.ProductName
	r.Params["display"] = v.DisplayVersion
	r.Params["build"] = strconv.FormatUint(uint64(v.Build), 10)
	r.Params["ubr"] = strconv.FormatUint(uint64(v.UBR), 10)
	r.Params["win11"] = strconv.FormatBool(v.IsWindows11())
	r.SourceURL = core.SrcMinSpecs
	r.Data = v
	return r
}

func osVersion(sys platform.System, deps map[string]core.Result) (platform.OSVersion, bool) {
	if r, ok := deps["C02"]; ok {
		if v, ok := r.Data.(platform.OSVersion); ok {
			return v, true
		}
	}
	v, err := sys.OSVersion()
	return v, err == nil
}

type TPMCheck struct{ base }

func (c *TPMCheck) DependsOn() []string { return []string{"C02"} }
func (c *TPMCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}
func (c *TPMCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	t, err := sys.TPM()
	if err != nil || t.Present == platform.Unknown {
		r := c.unknown(err)
		r.SourceURL = core.SrcTPM
		return r
	}
	v, _ := osVersion(sys, deps)
	win11 := v.IsWindows11()
	var r core.Result
	switch {
	case t.Present == platform.Yes && t.Version == "2.0":
		r = c.result(core.StatusOK, "tpm.ok")
	case win11:
		r = c.result(core.StatusFail, "tpm.off_win11")
		r.GuideIDs = []string{"G03"}
	default:
		r = c.result(core.StatusInfo, "tpm.off_win10")
		r.GuideIDs = []string{"G03"}
	}
	r.Params["version"] = t.Version
	r.SourceURL = core.SrcVAN9001
	r.Data = t
	return r
}

type SecureBootCheck struct{ base }

func (c *SecureBootCheck) DependsOn() []string { return []string{"C02"} }
func (c *SecureBootCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}
func (c *SecureBootCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	sb, err := sys.SecureBoot()
	if err != nil || sb == platform.Unknown {
		r := c.unknown(err)
		r.SourceURL = core.SrcSecureBoot
		return r
	}
	v, _ := osVersion(sys, deps)
	var r core.Result
	switch {
	case sb == platform.Yes:
		r = c.result(core.StatusOK, "secureboot.ok")
	case v.IsWindows11():
		r = c.result(core.StatusFail, "secureboot.off_win11")
		r.GuideIDs = []string{"G03"}
	default:
		r = c.result(core.StatusInfo, "secureboot.off_win10")
		r.GuideIDs = []string{"G03"}
	}
	r.SourceURL = core.SrcSecureBoot
	r.Data = sb
	return r
}

type VBSCheck struct{ base }

func (c *VBSCheck) Run(ctx context.Context, sys platform.System) core.Result {
	dg, err := sys.DeviceGuard()
	if err != nil || (dg.VBS == platform.Unknown && dg.HVCI == platform.Unknown) {
		r := c.unknown(err)
		r.SourceURL = core.SrcSecurityReqs
		return r
	}
	var r core.Result
	if dg.VBS == platform.Yes && dg.HVCI == platform.Yes {
		r = c.result(core.StatusOK, "vbs.ok")
	} else {
		r = c.result(core.StatusInfo, "vbs.off")
		r.GuideIDs = []string{"G04"}
		r.Params["vbs"] = triName(dg.VBS)
		r.Params["hvci"] = triName(dg.HVCI)
	}
	r.SourceURL = core.SrcSecurityReqs
	r.Data = dg
	return r
}

type IOMMUCheck struct{ base }

func (c *IOMMUCheck) Run(ctx context.Context, sys platform.System) core.Result {
	dg, err := sys.DeviceGuard()
	var r core.Result
	switch {
	case err != nil || dg.DMAProtection == platform.Unknown:
		r = c.unknown(err)
		r.MessageKey = "iommu.unknown"
	case dg.DMAProtection == platform.Yes:
		r = c.result(core.StatusOK, "iommu.ok")
	default:
		r = c.result(core.StatusInfo, "iommu.off")
		r.GuideIDs = []string{"G03"}
	}
	r.SourceURL = core.SrcSecurityReqs
	r.Data = dg.DMAProtection
	return r
}

func triName(t platform.TriState) string {
	switch t {
	case platform.Yes:
		return "yes"
	case platform.No:
		return "no"
	}
	return "unknown"
}

type OnDemand struct {
	Possible bool     `json:"possible"`
	Missing  []string `json:"missing"`
	Unknown  []string `json:"unknown"`
}

type OnDemandCheck struct{ base }

func (c *OnDemandCheck) DependsOn() []string {
	return []string{"C02", "C04", "C05", "C06", "C07"}
}
func (c *OnDemandCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}

func EvaluateOnDemand(v platform.OSVersion, tpm platform.TPMInfo, sb platform.TriState, dg platform.DeviceGuardInfo) OnDemand {
	od := OnDemand{Missing: []string{}, Unknown: []string{}}
	add := func(name string, t platform.TriState) {
		switch t {
		case platform.No:
			od.Missing = append(od.Missing, name)
		case platform.Unknown:
			od.Unknown = append(od.Unknown, name)
		}
	}
	if !(v.Major == 10 && v.Build >= Win11_25H2Build) {
		od.Missing = append(od.Missing, "win11_25h2")
	}
	tpmOK := platform.No
	if tpm.Present == platform.Yes && tpm.Version == "2.0" {
		tpmOK = platform.Yes
	} else if tpm.Present == platform.Unknown {
		tpmOK = platform.Unknown
	}
	add("tpm", tpmOK)
	add("secureboot", sb)
	add("iommu", dg.DMAProtection)
	add("vbs", dg.VBS)
	add("hvci", dg.HVCI)
	od.Possible = len(od.Missing) == 0
	return od
}

func (c *OnDemandCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	v, _ := osVersion(sys, deps)
	tpm, _ := sys.TPM()
	sb, _ := sys.SecureBoot()
	dg, _ := sys.DeviceGuard()
	od := EvaluateOnDemand(v, tpm, sb, dg)
	var r core.Result
	if od.Possible && len(od.Unknown) == 0 {
		r = c.result(core.StatusInfo, "ondemand.ready")
	} else {
		r = c.result(core.StatusInfo, "ondemand.not_ready")
	}
	r.Params["missing"] = strings.Join(od.Missing, ",")
	r.Params["unknown"] = strings.Join(od.Unknown, ",")
	r.SourceURL = core.SrcPreCheck
	r.Data = od
	return r
}

type VanguardState struct {
	VGC platform.ServiceInfo `json:"vgc"`
	VGK platform.ServiceInfo `json:"vgk"`
}

type VanguardCheck struct{ base }

func (c *VanguardCheck) Run(ctx context.Context, sys platform.System) core.Result {
	vgc, err1 := sys.Service("vgc")
	vgk, err2 := sys.Service("vgk")
	if err1 != nil && !errors.Is(err1, platform.ErrNotFound) {
		return c.unknown(err1)
	}
	var r core.Result
	switch {
	case !vgc.Exists:
		r = c.result(core.StatusInfo, "vanguard.not_installed")
	case vgc.StartType == platform.StartDisabled:
		r = c.result(core.StatusFail, "vanguard.vgc_disabled")
		r.FixIDs = []string{"F07"}
		r.GuideIDs = []string{"G01"}
	case err2 == nil && vgk.Exists && vgk.StartType == platform.StartDisabled:
		r = c.result(core.StatusFail, "vanguard.vgk_disabled")
		r.GuideIDs = []string{"G01"}
	case !vgk.Exists && err2 != nil && errors.Is(err2, platform.ErrNotFound):
		r = c.result(core.StatusWarn, "vanguard.vgk_missing")
		r.GuideIDs = []string{"G01"}
	case vgc.State == platform.StateRunning:
		r = c.result(core.StatusOK, "vanguard.running")
	default:
		r = c.result(core.StatusOK, "vanguard.normal_off")
	}
	r.Params["vgcState"] = string(vgc.State)
	r.Params["vgcStart"] = string(vgc.StartType)
	r.Params["vgkState"] = string(vgk.State)
	r.Params["vgkStart"] = string(vgk.StartType)
	r.SourceURL = core.SrcVAN57
	r.Data = VanguardState{VGC: vgc, VGK: vgk}
	return r
}

type DriversCheck struct{ base }

type DriverFinding struct {
	File    string `json:"file"`
	Vendor  string `json:"vendor"`
	Version string `json:"version"`
	Problem bool   `json:"problem"`
}

func CompareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(strings.TrimSpace(pa[i]))
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(strings.TrimSpace(pb[i]))
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func (c *DriversCheck) Run(ctx context.Context, sys platform.System) core.Result {
	var found []DriverFinding
	for _, d := range appcfg.Get().IncompatibleDrivers {
		df, err := sys.DriverFile(d.File)
		if err != nil || !df.Exists {
			continue
		}
		f := DriverFinding{File: d.File, Vendor: d.Vendor, Version: df.Version, Problem: true}
		if d.MinVersion != "" && df.Version != "" && CompareVersions(df.Version, d.MinVersion) >= 0 {
			f.Problem = false
		}
		if f.Problem {
			found = append(found, f)
		}
	}
	var r core.Result
	if len(found) == 0 {
		r = c.result(core.StatusOK, "drivers.ok")
	} else {
		r = c.result(core.StatusInfo, "drivers.found")
		var names []string
		for _, f := range found {
			names = append(names, f.File)
			r.AddDetail(core.StatusInfo, "drivers.item", map[string]string{"file": f.File, "vendor": f.Vendor, "version": f.Version})
		}
		r.Params["files"] = strings.Join(names, ", ")
		r.GuideIDs = []string{"G07"}
	}
	r.SourceURL = core.SrcOEMDriver
	r.Data = found
	return r
}

type SpecsCheck struct{ base }

func (c *SpecsCheck) Run(ctx context.Context, sys platform.System) core.Result {
	hw, err := sys.Hardware()
	if err != nil {
		return c.unknown(err)
	}
	r := c.result(core.StatusOK, "specs.ok")
	fail := func(key string, p map[string]string) {
		r.Status = core.StatusFail
		r.AddDetail(core.StatusFail, key, p)
		if r.MessageKey == "specs.ok" {
			r.MessageKey = key
		}
	}
	if hw.NativeArch != "" && hw.NativeArch != "amd64" {
		fail("specs.arm", map[string]string{"arch": hw.NativeArch})
	}
	if !hw.SSE42 && !hw.AVX {
		fail("specs.cpu", nil)
	}
	if hw.RAMBytes > 0 && hw.RAMBytes < 3900*1024*1024 {
		fail("specs.ram", map[string]string{"gb": strconv.FormatFloat(float64(hw.RAMBytes)/(1<<30), 'f', 1, 64)})
	}
	id := strings.ToLower(hw.Manufacturer + " " + hw.Model)
	for _, ind := range appcfg.Get().VMIndicators {
		if strings.Contains(id, ind) {
			fail("specs.vm", nil)
			r.SourceURL = core.SrcVAN138
			break
		}
	}
	if r.SourceURL == "" {
		r.SourceURL = core.SrcMinSpecs
	}
	r.Params["ramGb"] = strconv.FormatFloat(float64(hw.RAMBytes)/(1<<30), 'f', 1, 64)
	r.Data = hw
	return r
}

type PendingRebootCheck struct{ base }

func (c *PendingRebootCheck) Run(ctx context.Context, sys platform.System) core.Result {
	p, err := sys.PendingReboot()
	if err != nil {
		return c.unknown(err)
	}
	var r core.Result
	switch {
	case p.WindowsUpdate || p.CBS:
		r = c.result(core.StatusWarn, "reboot.pending")
		r.Steps = []core.Step{{Kind: core.StepInfo, ID: "reboot"}}
	case p.FileRename:
		r = c.result(core.StatusInfo, "reboot.maybe")
	default:
		r = c.result(core.StatusOK, "reboot.ok")
	}
	r.SourceURL = core.SrcVAN9006
	r.Data = p
	return r
}

type DiskCheck struct{ base }

func (c *DiskCheck) DependsOn() []string { return []string{"C01"} }
func (c *DiskCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}
func (c *DiskCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	inst := installFrom(sys, deps)
	target := inst.ValorantExe
	if target == "" {
		target = inst.ClientExe
	}
	if len(target) < 3 || target[1] != ':' {
		r := c.result(core.StatusUnknown, "disk.no_game")
		return r
	}
	root := target[:3]
	free, err := sys.DiskFree(root)
	if err != nil {
		return c.unknown(err)
	}
	gb := free / (1 << 30)
	var r core.Result
	if gb < appcfg.Get().FreeSpaceWarnGB {
		r = c.result(core.StatusWarn, "disk.low")
	} else {
		r = c.result(core.StatusOK, "disk.ok")
	}
	r.Params["drive"] = root[:2]
	r.Params["freeGb"] = strconv.FormatUint(gb, 10)
	r.Data = free
	return r
}

type TimeCheck struct{ base }

func (c *TimeCheck) DependsOn() []string { return []string{"C10"} }
func (c *TimeCheck) Run(ctx context.Context, sys platform.System) core.Result {
	return c.RunWith(ctx, sys, nil)
}
func (c *TimeCheck) RunWith(ctx context.Context, sys platform.System, deps map[string]core.Result) core.Result {
	var offsets []float64
	if r, ok := deps["C10"]; ok {
		if d, ok := r.Data.(ServersData); ok {
			for _, h := range d.Reports {
				for _, p := range h.Probes {
					if !p.ServerDate.IsZero() && !p.MeasuredAt.IsZero() {
						offsets = append(offsets, p.MeasuredAt.Sub(p.ServerDate).Seconds())
					}
				}
			}
		}
	}
	if len(offsets) == 0 {
		r := c.result(core.StatusUnknown, "time.no_reference")
		r.SourceURL = core.SrcVAN81
		return r
	}
	sort.Float64s(offsets)
	med := offsets[len(offsets)/2]
	abs := med
	if abs < 0 {
		abs = -abs
	}
	var r core.Result
	if abs > 90 {
		r = c.result(core.StatusFail, "time.wrong")
		r.Steps = []core.Step{{Kind: core.StepInfo, ID: "time_sync"}}
	} else {
		r = c.result(core.StatusOK, "time.ok")
	}
	r.Params["offset"] = strconv.Itoa(int(abs))
	r.SourceURL = core.SrcVAN81
	r.Data = time.Duration(med * float64(time.Second))
	return r
}
