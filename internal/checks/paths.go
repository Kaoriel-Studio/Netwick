package checks

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/kaoriel-studio/netwick/internal/core"
	"github.com/kaoriel-studio/netwick/internal/platform"
)

func WinJoin(parts ...string) string {
	var out []string
	for i, p := range parts {
		p = strings.ReplaceAll(p, "/", `\`)
		if i > 0 {
			p = strings.Trim(p, `\`)
		} else {
			p = strings.TrimRight(p, `\`)
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, `\`)
}

func WinDir(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	if i := strings.LastIndex(p, `\`); i > 0 {
		return p[:i]
	}
	return p
}

func WinBase(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	if i := strings.LastIndex(p, `\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

type Install struct {
	ClientExe   string `json:"clientExe"`
	ValorantExe string `json:"valorantExe"`
	VgcExe      string `json:"vgcExe"`
}

func (i Install) Binaries() []string {
	var out []string
	for _, p := range []string{i.ClientExe, i.ValorantExe, i.VgcExe} {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

var yamlInstallRe = regexp.MustCompile(`product_install_full_path:\s*"?([^"\r\n]+)"?`)

func exists(sys platform.System, p string) bool {
	if p == "" {
		return false
	}
	fi, err := sys.Stat(p)
	return err == nil && !fi.IsDir
}

func ResolveInstall(sys platform.System) Install {
	var inst Install
	programData := sys.Env("ProgramData")
	if programData != "" {
		if b, err := sys.ReadFile(WinJoin(programData, "Riot Games", "RiotClientInstalls.json"), 1<<20); err == nil {
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				for _, k := range []string{"rc_default", "rc_live", "rc_beta"} {
					if s, ok := m[k].(string); ok {
						p := WinJoin(s)
						if strings.EqualFold(WinBase(p), "RiotClientServices.exe") && exists(sys, p) {
							inst.ClientExe = p
							break
						}
					}
				}
			}
		}
		if b, err := sys.ReadFile(WinJoin(programData, "Riot Games", "Metadata", "valorant.live", "valorant.live.product_settings.yaml"), 1<<20); err == nil {
			for _, m := range yamlInstallRe.FindAllStringSubmatch(string(b), -1) {
				cand := WinJoin(strings.TrimSpace(m[1]), "VALORANT.exe")
				if exists(sys, cand) {
					inst.ValorantExe = cand
					break
				}
			}
		}
	}
	for _, root := range sys.FixedDrives() {
		if inst.ClientExe == "" {
			if p := WinJoin(root, "Riot Games", "Riot Client", "RiotClientServices.exe"); exists(sys, p) {
				inst.ClientExe = p
			}
		}
		if inst.ValorantExe == "" {
			if p := WinJoin(root, "Riot Games", "VALORANT", "live", "VALORANT.exe"); exists(sys, p) {
				inst.ValorantExe = p
			}
		}
	}
	if inst.ClientExe == "" {
		if la := sys.Env("LOCALAPPDATA"); la != "" {
			if p := WinJoin(la, "Riot Games", "Riot Client", "RiotClientServices.exe"); exists(sys, p) {
				inst.ClientExe = p
			}
		}
	}
	if pf := sys.Env("ProgramFiles"); pf != "" {
		if p := WinJoin(pf, "Riot Vanguard", "vgc.exe"); exists(sys, p) {
			inst.VgcExe = p
		}
	}
	return inst
}

func installFrom(sys platform.System, deps map[string]core.Result) Install {
	if r, ok := deps["C01"]; ok {
		if inst, ok := r.Data.(Install); ok {
			return inst
		}
	}
	return ResolveInstall(sys)
}
