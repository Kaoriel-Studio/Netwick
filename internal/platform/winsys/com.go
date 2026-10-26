//go:build windows

package winsys

import (
	"errors"
	"runtime"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/kaoriel-studio/netwick/internal/platform"
)

func withCOM(fn func() error) error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
			var oe *ole.OleError
			if !errors.As(err, &oe) || (oe.Code() != 1 && oe.Code() != 0) {
				done <- err
				return
			}
		}
		defer ole.CoUninitialize()
		done <- fn()
	}()
	return <-done
}

func createDispatch(progID string) (*ole.IDispatch, error) {
	unk, err := oleutil.CreateObject(progID)
	if err != nil {
		return nil, err
	}
	defer unk.Release()
	return unk.QueryInterface(ole.IID_IDispatch)
}

func variantValue(v *ole.VARIANT) any {
	if v == nil {
		return nil
	}
	if v.VT&ole.VT_ARRAY != 0 {
		arr := v.ToArray()
		if arr == nil {
			return []any{}
		}
		return arr.ToValueArray()
	}
	return v.Value()
}

func wmiQuery(namespace, query string, props []string) ([]map[string]any, error) {
	var rows []map[string]any
	err := withCOM(func() error {
		loc, err := createDispatch("WbemScripting.SWbemLocator")
		if err != nil {
			return err
		}
		defer loc.Release()
		svcRaw, err := oleutil.CallMethod(loc, "ConnectServer", nil, namespace)
		if err != nil {
			return err
		}
		svc := svcRaw.ToIDispatch()
		defer svc.Release()
		resRaw, err := oleutil.CallMethod(svc, "ExecQuery", query)
		if err != nil {
			return err
		}
		res := resRaw.ToIDispatch()
		defer res.Release()
		return oleutil.ForEach(res, func(v *ole.VARIANT) error {
			item := v.ToIDispatch()
			if item == nil {
				return nil
			}
			row := map[string]any{}
			for _, p := range props {
				val, err := oleutil.GetProperty(item, p)
				if err != nil {
					continue
				}
				row[p] = variantValue(val)
				_ = val.Clear()
			}
			rows = append(rows, row)
			if len(rows) > 64 {
				return errors.New("too many rows")
			}
			return nil
		})
	})
	return rows, err
}

const (
	fwActionBlock = 0
	fwActionAllow = 1
	fwDirIn       = 1
	fwDirOut      = 2
	fwProfilesAll = 0x7FFFFFFF
)

func withRules(fn func(policy, rules *ole.IDispatch) error) error {
	return withCOM(func() error {
		policy, err := createDispatch("HNetCfg.FwPolicy2")
		if err != nil {
			return err
		}
		defer policy.Release()
		rv, err := oleutil.GetProperty(policy, "Rules")
		if err != nil {
			return err
		}
		rules := rv.ToIDispatch()
		defer rules.Release()
		return fn(policy, rules)
	})
}

func propString(d *ole.IDispatch, name string) string {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return ""
	}
	defer v.Clear()
	s, _ := v.Value().(string)
	return s
}

func propInt(d *ole.IDispatch, name string) int64 {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return -1
	}
	defer v.Clear()
	n, _ := asInt(v.Value())
	return n
}

func propBool(d *ole.IDispatch, name string) bool {
	v, err := oleutil.GetProperty(d, name)
	if err != nil {
		return false
	}
	defer v.Clear()
	b, _ := v.Value().(bool)
	return b
}

func (s *System) FirewallRules() ([]platform.FirewallRule, error) {
	var out []platform.FirewallRule
	err := withRules(func(_, rules *ole.IDispatch) error {
		return oleutil.ForEach(rules, func(v *ole.VARIANT) error {
			r := v.ToIDispatch()
			if r == nil {
				return nil
			}
			app := propString(r, "ApplicationName")
			if app == "" {
				return nil
			}
			out = append(out, platform.FirewallRule{
				Name:        propString(r, "Name"),
				Application: app,
				Enabled:     propBool(r, "Enabled"),
				Block:       propInt(r, "Action") == fwActionBlock,
				Inbound:     propInt(r, "Direction") == fwDirIn,
			})
			if len(out) > 20000 {
				return errors.New("too many rules")
			}
			return nil
		})
	})
	return out, err
}

func (s *System) FirewallEnabled() (platform.TriState, error) {
	state := platform.Unknown
	err := withCOM(func() error {
		policy, err := createDispatch("HNetCfg.FwPolicy2")
		if err != nil {
			return err
		}
		defer policy.Release()
		cur := propInt(policy, "CurrentProfileTypes")
		if cur <= 0 {
			return nil
		}
		state = platform.No
		for _, p := range []int64{1, 2, 4} {
			if cur&p == 0 {
				continue
			}
			v, err := oleutil.GetProperty(policy, "FirewallEnabled", p)
			if err != nil {
				continue
			}
			if b, _ := v.Value().(bool); b {
				state = platform.Yes
			}
			_ = v.Clear()
		}
		return nil
	})
	return state, err
}

func ruleExists(rules *ole.IDispatch, name string) bool {
	v, err := oleutil.CallMethod(rules, "Item", name)
	if err != nil {
		return false
	}
	if d := v.ToIDispatch(); d != nil {
		d.Release()
	}
	return true
}

func (a *Actions) AddFirewallAllow(list []platform.FirewallAllow) error {
	return withRules(func(_, rules *ole.IDispatch) error {
		for _, r := range list {
			for _, dir := range []int{fwDirIn, fwDirOut} {
				rule, err := createDispatch("HNetCfg.FWRule")
				if err != nil {
					return err
				}
				set := func(k string, v any) {
					if err == nil {
						_, err = oleutil.PutProperty(rule, k, v)
					}
				}
				set("Name", r.Name)
				set("Description", "Added by Netwick. Remove with the Undo button in Netwick.")
				set("ApplicationName", r.Application)
				set("Action", fwActionAllow)
				set("Direction", dir)
				set("Profiles", fwProfilesAll)
				set("Enabled", true)
				if err == nil {
					_, err = oleutil.CallMethod(rules, "Add", rule)
				}
				rule.Release()
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Actions) RemoveFirewallRules(names []string) error {
	return withRules(func(_, rules *ole.IDispatch) error {
		for _, n := range names {
			if !strings.HasPrefix(n, "Netwick — ") {
				return platform.ErrDenied
			}
			for i := 0; i < 10 && ruleExists(rules, n); i++ {
				if _, err := oleutil.CallMethod(rules, "Remove", n); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (a *Actions) FirewallRuleExists(name string) (bool, error) {
	found := false
	err := withRules(func(_, rules *ole.IDispatch) error {
		found = ruleExists(rules, name)
		return nil
	})
	return found, err
}
