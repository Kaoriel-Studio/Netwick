package worker

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaoriel-studio/netwick/internal/fixes"
)

const ActionUndo = "UNDO"
const MaxActions = 10

type Args struct {
	Pipe             string
	Actions          []string
	DNSAdapterLUID   uint64
	SkipRestorePoint bool
}

var pipeRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
var luidRe = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

var (
	ErrUsage   = errors.New("invalid worker arguments")
	ErrUnknown = errors.New("unknown action")
)

func privileged(id string) bool {
	for _, p := range fixes.PrivilegedIDs {
		if p == id {
			return true
		}
	}
	return false
}

func ParseArgs(argv []string) (Args, error) {
	var a Args
	if len(argv) == 0 || argv[0] != "--worker" {
		return a, ErrUsage
	}
	seen := map[string]bool{}
	for _, arg := range argv[1:] {
		if len(arg) > 256 {
			return a, ErrUsage
		}
		name, val, hasVal := strings.Cut(arg, "=")
		if seen[name] {
			return a, ErrUsage
		}
		seen[name] = true
		switch name {
		case "--pipe":
			if !hasVal || !pipeRe.MatchString(val) {
				return a, ErrUsage
			}
			a.Pipe = val
		case "--actions":
			if !hasVal || val == "" {
				return a, ErrUsage
			}
			parts := strings.Split(val, ",")
			if len(parts) > MaxActions {
				return a, ErrUsage
			}
			dup := map[string]bool{}
			for _, p := range parts {
				if dup[p] {
					return a, ErrUsage
				}
				dup[p] = true
				if p == ActionUndo {
					if len(parts) != 1 {
						return a, ErrUsage
					}
				} else if !privileged(p) {
					return a, ErrUnknown
				}
				a.Actions = append(a.Actions, p)
			}
		case "--dns-adapter-luid":
			if !hasVal || !luidRe.MatchString(val) {
				return a, ErrUsage
			}
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil || n == 0 {
				return a, ErrUsage
			}
			a.DNSAdapterLUID = n
		case "--skip-restore-point":
			if hasVal {
				return a, ErrUsage
			}
			a.SkipRestorePoint = true
		default:
			return a, ErrUsage
		}
	}
	if a.Pipe == "" || len(a.Actions) == 0 {
		return a, ErrUsage
	}
	needLUID := false
	for _, x := range a.Actions {
		if x == "F04" {
			needLUID = true
		}
	}
	if needLUID != (a.DNSAdapterLUID != 0) {
		return a, ErrUsage
	}
	return a, nil
}

func (a Args) Build() []string {
	out := []string{"--worker", "--pipe=" + a.Pipe, "--actions=" + strings.Join(a.Actions, ",")}
	if a.DNSAdapterLUID != 0 {
		out = append(out, "--dns-adapter-luid="+strconv.FormatUint(a.DNSAdapterLUID, 10))
	}
	if a.SkipRestorePoint {
		out = append(out, "--skip-restore-point")
	}
	return out
}
