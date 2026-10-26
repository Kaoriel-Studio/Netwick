package fixes

import (
	"sort"

	"github.com/kaoriel-studio/netwick/internal/core"
)

type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

type Fix struct {
	ID             string `json:"id"`
	SourceURL      string `json:"sourceUrl"`
	RequiresAdmin  bool   `json:"requiresAdmin"`
	RequiresReboot bool   `json:"requiresReboot"`
	Reversible     bool   `json:"reversible"`
	Risk           Risk   `json:"risk"`
	RestorePoint   bool   `json:"restorePoint"`
	Scenario       string `json:"scenario"`
}

var Catalog = map[string]Fix{
	"F01": {ID: "F01", SourceURL: core.SrcVALErrorCodes, Risk: RiskLow},
	"F02": {ID: "F02", SourceURL: core.SrcDNS, RequiresAdmin: true, Risk: RiskLow},
	"F03": {ID: "F03", SourceURL: core.SrcDNS, RequiresAdmin: true, Risk: RiskLow},
	"F04": {ID: "F04", SourceURL: core.SrcDNS, RequiresAdmin: true, Reversible: true, Risk: RiskMedium, RestorePoint: true},
	"F05": {ID: "F05", SourceURL: core.SrcConnection, RequiresAdmin: true, RequiresReboot: true, Risk: RiskHigh, RestorePoint: true},
	"F06": {ID: "F06", SourceURL: core.SrcFirewall, RequiresAdmin: true, Reversible: true, Risk: RiskLow},
	"F07": {ID: "F07", SourceURL: core.SrcVAN57, RequiresAdmin: true, RequiresReboot: true, Reversible: true, Risk: RiskMedium, RestorePoint: true},
	"F08": {ID: "F08", SourceURL: core.SrcVAN128, RequiresAdmin: true, RequiresReboot: true, Reversible: true, Risk: RiskMedium, RestorePoint: true, Scenario: "VAN 128"},
	"F09": {ID: "F09", SourceURL: core.SrcKiller, RequiresAdmin: true, Reversible: true, Risk: RiskMedium, RestorePoint: true, Scenario: "VAN 68"},
	"F10": {ID: "F10", SourceURL: "", RequiresAdmin: true, Reversible: true, Risk: RiskLow},
}

var PrivilegedIDs = []string{"F02", "F03", "F04", "F05", "F06", "F07", "F08", "F09", "F10"}

func Get(id string) (Fix, bool) {
	f, ok := Catalog[id]
	return f, ok
}

func IDs() []string {
	ids := make([]string, 0, len(Catalog))
	for id := range Catalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type Opener string

const (
	OpenWindowsUpdate     Opener = "windowsupdate"
	OpenMsconfig          Opener = "msconfig"
	OpenTaskManager       Opener = "taskmgr"
	OpenAppsFeatures      Opener = "appsfeatures"
	OpenCoreIsolation     Opener = "coreisolation"
	OpenExploitProtection Opener = "exploitprotection"
	OpenFirewallAdvanced  Opener = "firewall"
	OpenProxySettings     Opener = "proxy"
	OpenVPNSettings       Opener = "vpn"
	OpenNetworkStatus     Opener = "network"
	OpenDateTime          Opener = "datetime"
	OpenSystemInfo        Opener = "msinfo"
	OpenDeviceManager     Opener = "devmgmt"
)

var Openers = []Opener{OpenWindowsUpdate, OpenMsconfig, OpenTaskManager, OpenAppsFeatures, OpenCoreIsolation, OpenExploitProtection, OpenFirewallAdvanced, OpenProxySettings, OpenVPNSettings, OpenNetworkStatus, OpenDateTime, OpenSystemInfo, OpenDeviceManager}

func ValidOpener(o string) bool {
	for _, x := range Openers {
		if string(x) == o {
			return true
		}
	}
	return false
}

type Guide struct {
	ID        string   `json:"id"`
	SourceURL string   `json:"sourceUrl"`
	Opens     []Opener `json:"opens"`
	Steps     int      `json:"steps"`
	Warn      bool     `json:"warn"`
}

var Guides = map[string]Guide{
	"G01": {ID: "G01", SourceURL: core.SrcUninstallVanguard, Opens: []Opener{OpenAppsFeatures}, Steps: 4},
	"G02": {ID: "G02", SourceURL: core.SrcCleanBoot, Opens: []Opener{OpenMsconfig, OpenTaskManager}, Steps: 6},
	"G03": {ID: "G03", SourceURL: core.SrcTPM, Opens: []Opener{OpenSystemInfo}, Steps: 4, Warn: true},
	"G04": {ID: "G04", SourceURL: core.SrcSecurityReqs, Opens: []Opener{OpenCoreIsolation}, Steps: 3},
	"G05": {ID: "G05", SourceURL: core.SrcVAN9002, Opens: []Opener{OpenExploitProtection}, Steps: 4, Warn: true},
	"G06": {ID: "G06", SourceURL: core.SrcVAN9006, Opens: []Opener{OpenWindowsUpdate}, Steps: 3},
	"G07": {ID: "G07", SourceURL: core.SrcOEMDriver, Opens: []Opener{OpenDeviceManager}, Steps: 3, Warn: true},
	"G08": {ID: "G08", SourceURL: core.SrcConnection, Opens: []Opener{OpenVPNSettings, OpenProxySettings}, Steps: 3},
	"G09": {ID: "G09", SourceURL: core.SrcCannotUpdate, Steps: 4, Warn: true},
	"G10": {ID: "G10", SourceURL: core.SrcConnection, Opens: []Opener{OpenNetworkStatus}, Steps: 3},
	"G11": {ID: "G11", SourceURL: core.SrcVAN57, Opens: []Opener{OpenAppsFeatures}, Steps: 3},
	"G12": {ID: "G12", SourceURL: core.SrcVAN152, Steps: 2},
	"G13": {ID: "G13", SourceURL: core.SrcRepairTool, Steps: 3},
	"G14": {ID: "G14", SourceURL: core.SrcFirewall, Opens: []Opener{OpenFirewallAdvanced}, Steps: 4},
}

func GetGuide(id string) (Guide, bool) {
	g, ok := Guides[id]
	return g, ok
}

var InfoSteps = map[string]string{
	"reboot":                    core.SrcVAN57,
	"time_sync":                 core.SrcVAN81,
	"usb":                       core.SrcVAN9101,
	"practice_range":            core.SrcVAN9101,
	"sign_out_other":            core.SrcVALErrorCodes,
	"uninstall_driver_software": core.SrcIncompatibleSW,
	"delete_sys":                core.SrcIncompatibleSW,
	"riot_id":                   core.SrcVALErrorCodes,
	"check_email":               core.SrcVALErrorCodes,
	"server_status":             core.SrcServerStatus,
	"wait":                      core.SrcServerStatus,
}

func StepExists(s core.Step) bool {
	switch s.Kind {
	case core.StepFix:
		_, ok := Catalog[s.ID]
		return ok
	case core.StepGuide:
		_, ok := Guides[s.ID]
		return ok
	case core.StepInfo:
		_, ok := InfoSteps[s.ID]
		return ok
	case core.StepArticle:
		return s.ID == "official"
	case core.StepCheck:
		return len(s.ID) == 3 && s.ID[0] == 'C' && s.ID >= "C01" && s.ID <= "C21"
	}
	return false
}
