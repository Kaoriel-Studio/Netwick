package fake

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kaoriel-studio/netwick/internal/platform"
	"github.com/kaoriel-studio/netwick/internal/probe"
)

const (
	AllOK          = "all-ok"
	OwnerRealRun   = "owner-real-run"
	NoInternet     = "no-internet"
	ISPCutsSomeIPs = "isp-cuts-some-ips"
	TPMOffWin11    = "tpm-off-win11"
	SecureBootOff  = "secure-boot-off"
	VGCDisabled    = "vgc-disabled"
	FirewallBlocks = "firewall-blocks"
	VPNOn          = "vpn-on"
	VAN68          = "van-68"
	VAN128         = "van-128"
	OldHosts       = "old-hosts-entries"
	VAL43          = "val-43"
	RiotIncident   = "riot-incident"
	LaptopSaver    = "laptop-saver"
	WifiNoise      = "wifi-noise"
)

var Scenarios = []string{AllOK, VAL43, OwnerRealRun, NoInternet, ISPCutsSomeIPs, RiotIncident, LaptopSaver, WifiNoise, TPMOffWin11, SecureBootOff, VGCDisabled, FirewallBlocks, VPNOn, VAN68, VAN128, OldHosts}

func Valid(name string) bool {
	for _, s := range Scenarios {
		if s == name {
			return true
		}
	}
	return false
}

const (
	userProfile = `C:\Users\Player`
	clientExe   = `C:\Riot Games\Riot Client\RiotClientServices.exe`
	gameDir     = `E:\Riot Games\VALORANT\live`
	vgcExe      = `C:\Program Files\Riot Vanguard\vgc.exe`
	wifiLUID    = 0x47000000000001
	ethLUID     = 0x6000000000002
)

const (
	riotClientLocal = userProfile + `\AppData\Local\Riot Games\Riot Client`
	logDir          = riotClientLocal + `\Logs\Riot Client Logs`
	clearStatus     = `{"id":"EU","name":"Europe","maintenances":[],"incidents":[]}`
	incidentStatus  = `{"id":"EU","name":"Europe","maintenances":[],"incidents":[{"titles":[{"locale":"en_US","content":"Players may be unable to log in or may experience disconnects"},{"locale":"ru_RU","content":"Игроки могут не входить в игру или терять соединение"}]}]}`
)

const defaultHosts = "# Copyright (c) 1993-2009 Microsoft Corp.\r\n#\r\n# This is a sample HOSTS file used by Microsoft TCP/IP for Windows.\r\n#\r\n# localhost name resolution is handled within DNS itself.\r\n#\t127.0.0.1       localhost\r\n#\t::1             localhost\r\n127.0.0.1 local.example-tool.test\r\n"

func healthyPools() (map[string][]string, map[string]ipState) {
	pools := map[string][]string{
		"valorant.secure.dyn.riotcdn.net":    {"65.9.46.10", "65.9.46.11", "65.9.46.12"},
		"riot-client.secure.dyn.riotcdn.net": {"65.9.46.10", "65.9.46.11", "65.9.46.12"},
		"clientconfig.rpg.riotgames.com":     {"104.18.156.37", "104.18.157.37"},
		"auth.riotgames.com":                 {"104.16.119.50", "104.16.120.50"},
		"entitlements.auth.riotgames.com":    {"104.16.56.21", "104.16.57.21"},
		"data.riotgames.com":                 {"104.18.40.11", "104.18.41.11"},
	}
	ips := map[string]ipState{}
	lat := 38
	for _, h := range sortedKeys(pools) {
		for _, ip := range pools[h] {
			ips[ip] = ipState{status: probe.StatusOk, latency: lat}
			lat += 3
		}
	}
	return pools, ips
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func logLines(host string, n int, at time.Time) string {
	var b strings.Builder
	b.WriteString(at.Format("2006-01-02T15:04:05.000") + "| OKAY| Riot Client starting\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s| WARN| Failure to connect to URL https://%s/api/v1/telemetry: SSL failure\n", at.Add(time.Duration(i)*time.Second).Format("2006-01-02T15:04:05.000"), host)
	}
	return b.String()
}

func New(scenario string) *System {
	if !Valid(scenario) {
		scenario = AllOK
	}
	now := time.Now()
	pools, ips := healthyPools()
	s := &System{
		scenario: scenario,
		env: map[string]string{
			"ProgramData":  `C:\ProgramData`,
			"LOCALAPPDATA": userProfile + `\AppData\Local`,
			"APPDATA":      userProfile + `\AppData\Roaming`,
			"USERPROFILE":  userProfile,
			"USERNAME":     "Player",
			"COMPUTERNAME": "PLAYER-PC",
			"ProgramFiles": `C:\Program Files`,
			"SystemRoot":   `C:\Windows`,
			"SystemDrive":  `C:`,
		},
		files:      map[string]file{},
		drives:     []string{`C:\`, `E:\`},
		osv:        platform.OSVersion{Major: 10, Minor: 0, Build: 26100, UBR: 4652, ProductName: "Windows 11 Home", DisplayVersion: "24H2"},
		tpm:        platform.TPMInfo{Present: platform.Yes, Version: "2.0"},
		secureBoot: platform.Yes,
		dg:         platform.DeviceGuardInfo{VBS: platform.Yes, HVCI: platform.No, DMAProtection: platform.Yes},
		services: map[string]platform.ServiceInfo{
			"vgc": {Name: "vgc", DisplayName: "vgc", Exists: true, State: platform.StateStopped, StartType: platform.StartManual},
			"vgk": {Name: "vgk", DisplayName: "vgk", Exists: true, State: platform.StateRunning, StartType: platform.StartSystem},
		},
		adapters: []platform.Adapter{
			{LUID: ethLUID, Name: "Ethernet", Description: "Realtek PCIe 2.5GbE Family Controller", IfType: platform.IfTypeEthernet, Up: true, HasGateway: true, DNS: []string{"192.168.1.1"}},
			{LUID: wifiLUID, Name: "Wi-Fi", Description: "Intel(R) Wi-Fi 6E AX211 160MHz", IfType: platform.IfTypeWiFi, Up: false, DNS: []string{}},
		},
		fwEnabled: platform.Yes,
		procs: []platform.Process{
			{PID: 4, Name: "System"}, {PID: 812, Name: "explorer.exe"}, {PID: 1440, Name: "vgtray.exe"},
		},
		drivers:   map[string]platform.DriverFile{},
		hw:        platform.Hardware{NativeArch: "amd64", SSE42: true, AVX: true, RAMBytes: 16 << 30, Manufacturer: "Micro-Star International Co., Ltd.", Model: "MS-7D75"},
		diskFree:  map[string]uint64{"C": 120 << 30, "E": 410 << 30},
		dnsOK:     true,
		pools:     pools,
		ips:       ips,
		crashDump: platform.RegDWORD{Exists: true, Value: 7},
		dhcp:      map[uint64]bool{ethLUID: true, wifiLUID: true},
		power:     platform.PowerInfo{Plan: platform.PlanHigh, OnBattery: platform.No, GameMode: platform.Yes},
	}
	s.putFile(`C:\ProgramData\Riot Games\RiotClientInstalls.json`, `{"associated_client":{"E:/Riot Games/VALORANT/live/":"C:/Riot Games/Riot Client/RiotClientServices.exe"},"patchlines":{"KeystoneFoundationLiveWin":"C:/Riot Games/Riot Client/RiotClientServices.exe"},"rc_default":"C:/Riot Games/Riot Client/RiotClientServices.exe","rc_live":"C:/Riot Games/Riot Client/RiotClientServices.exe"}`, now)
	s.putFile(`C:\ProgramData\Riot Games\Metadata\valorant.live\valorant.live.product_settings.yaml`, "product_install_full_path: \"E:/Riot Games/VALORANT/live\"\nproduct_install_root: \"E:/Riot Games\"\n", now)
	s.putFile(clientExe, "MZ", now)
	s.putFile(gameDir+`\VALORANT.exe`, "MZ", now)
	s.putFile(vgcExe, "MZ", now)
	s.putFile(hostsPath, defaultHosts, now)
	s.putFile(logDir+`\2026-10-25T20-11-02_1234_Riot Client.log`, logLines("", 0, now.Add(-2*time.Hour)), now.Add(-2*time.Hour))
	s.putFile(riotClientLocal+`\Cache\data_0`, "cache", now)
	s.putFile(riotClientLocal+`\Cache\index`, "cache", now)
	s.putFile(riotClientLocal+`\Crashpad\settings.dat`, "crashpad", now)
	s.putFile(userProfile+`\AppData\Local\VALORANT\Saved\Crashes\crash-1\report.txt`, "crash", now)
	s.putFile(riotClientLocal+`\Data\RiotGamesPrivateSettings.yaml`, "session", now)
	s.putFile(riotClientLocal+`\Config\RiotClientSettings.yaml`, "settings", now)
	s.status = map[string]string{"valorant": clearStatus, "riot_client": clearStatus}

	switch scenario {
	case OwnerRealRun:
		s.osv = platform.OSVersion{Major: 10, Build: 26100, UBR: 6899, ProductName: "Windows 11 Pro", DisplayVersion: "24H2"}
		s.adapters[0].DNS = []string{"8.8.8.8", "8.8.4.4"}
		s.dhcp[ethLUID] = false
		s.pools["valorant.secure.dyn.riotcdn.net"] = []string{"65.9.46.9", "65.9.46.10", "65.9.46.11", "65.9.46.12"}
		s.pools["riot-client.secure.dyn.riotcdn.net"] = []string{"65.9.46.9", "65.9.46.10", "65.9.46.11", "65.9.46.12"}
		s.ips["65.9.46.9"] = ipState{status: probe.StatusNoTLS, cause: probe.CauseTimeout}
		s.ips["104.18.156.37"] = ipState{status: probe.StatusNoTLS, cause: probe.CauseTimeout}
		s.ips["104.16.119.50"] = ipState{status: probe.StatusNoTLS, cause: probe.CauseReset}
		s.ips["104.18.40.11"] = ipState{status: probe.StatusNoTLS, cause: probe.CauseTimeout}
		s.ips["104.18.41.11"] = ipState{status: probe.StatusNoTLS, cause: probe.CauseTimeout}
		s.putFile(hostsPath, defaultHosts+"\r\n# >>> CYPHERLINK-ROUTE >>> (added by CypherLink)\r\n104.16.56.21     entitlements.auth.riotgames.com\r\n104.18.157.37    clientconfig.rpg.riotgames.com\r\n104.16.120.50    auth.riotgames.com\r\n# <<< CYPHERLINK-ROUTE <<<\r\n", now)
		s.putFile(logDir+`\2026-10-25T21-40-10_5678_Riot Client.log`, logLines("data.riotgames.com", 63, now.Add(-time.Hour)), now.Add(-time.Hour))
	case NoInternet:
		s.dnsOK = false
	case ISPCutsSomeIPs:
		for _, h := range []string{"valorant.secure.dyn.riotcdn.net", "riot-client.secure.dyn.riotcdn.net"} {
			for _, ip := range s.pools[h] {
				s.ips[ip] = ipState{status: probe.StatusStalled, cause: probe.CauseTimeout}
			}
		}
		s.putFile(logDir+`\2026-10-25T21-40-10_5678_Riot Client.log`, logLines("valorant.secure.dyn.riotcdn.net", 12, now.Add(-time.Hour)), now.Add(-time.Hour))
	case TPMOffWin11:
		s.tpm = platform.TPMInfo{Present: platform.No}
		s.dg.DMAProtection = platform.No
	case SecureBootOff:
		s.secureBoot = platform.No
	case VGCDisabled:
		s.services["vgc"] = platform.ServiceInfo{Name: "vgc", DisplayName: "vgc", Exists: true, State: platform.StateStopped, StartType: platform.StartDisabled}
	case FirewallBlocks:
		s.rules = append(s.rules, platform.FirewallRule{Name: "Block VALORANT", Application: gameDir + `\VALORANT.exe`, Enabled: true, Block: true, Inbound: false})
	case VPNOn:
		s.adapters = append(s.adapters, platform.Adapter{LUID: 0x83000000000003, Name: "wg0", Description: "WireGuard Tunnel", IfType: platform.IfTypeTunnel, Up: true, HasGateway: true, DNS: []string{"10.2.0.1"}})
		s.proxy = platform.ProxyInfo{Enabled: true, Server: "127.0.0.1:8080"}
	case VAN68:
		s.services["killer network service"] = platform.ServiceInfo{Name: "Killer Network Service", DisplayName: "Killer Network Service", Exists: true, State: platform.StateRunning, StartType: platform.StartAutomatic}
		s.adapters[0].Description = "Killer E3100G 2.5 Gigabit Ethernet Controller"
		s.clock = 4 * time.Minute
	case VAN128:
		s.pending = platform.PendingReboot{WindowsUpdate: true}
		s.crashDump = platform.RegDWORD{Exists: true, Value: 7}
	case VAL43:
		s.pools["valorant.secure.dyn.riotcdn.net"] = []string{"65.9.46.9", "65.9.46.10", "65.9.46.11", "65.9.46.12"}
		s.pools["clientconfig.rpg.riotgames.com"] = []string{"104.18.155.37", "104.18.156.37", "104.18.157.37"}
		s.ips["65.9.46.9"] = ipState{status: probe.StatusStalled, cause: probe.CauseTimeout}
		s.ips["104.18.155.37"] = ipState{status: probe.StatusStalled, cause: probe.CauseReset}
		s.services["bonjour service"] = platform.ServiceInfo{Name: "Bonjour Service", DisplayName: "Bonjour Service", Exists: true, State: platform.StateRunning, StartType: platform.StartAutomatic}
		s.procs = append(s.procs, platform.Process{PID: 2210, Name: "RiotClientServices.exe"}, platform.Process{PID: 2290, Name: "RiotClientUxRender.exe"})
		s.putFile(logDir+`\2026-10-25T21-40-10_5678_Riot Client.log`, logLines("valorant.secure.dyn.riotcdn.net", 17, now.Add(-time.Hour))+logLines("clientconfig.rpg.riotgames.com", 9, now.Add(-time.Hour)), now.Add(-time.Hour))
	case RiotIncident:
		s.status["valorant"] = incidentStatus
	case LaptopSaver:
		s.power = platform.PowerInfo{Plan: platform.PlanPowerSaver, OnBattery: platform.Yes, GameMode: platform.No}
	case WifiNoise:
		s.adapters[0].Up = false
		s.adapters[1].Up = true
		s.adapters[1].HasGateway = true
		s.adapters[1].DNS = []string{"192.168.1.1"}
	case OldHosts:
		s.putFile(hostsPath, defaultHosts+"\r\n# >>> NETWICK >>> (managed routes, added 2026-03-01 — safe to delete)\r\n65.9.46.11         valorant.secure.dyn.riotcdn.net\r\n# <<< NETWICK <<<\r\n# >>> CYPHERLINK-CDN >>>\r\n104.16.120.50 auth.riotgames.com\r\n# <<< CYPHERLINK-CDN <<<\r\n", now)
		s.ips["65.9.46.11"] = ipState{status: probe.StatusNoTLS, cause: probe.CauseTimeout}
	}

	dir := filepath.Join(os.TempDir(), "netwick-demo-backups")
	_ = os.MkdirAll(dir, 0o700)
	s.backupDir = dir
	return s
}
