package undo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/platform"
)

const RulePrefix = "Netwick — "

var AllowedRuleApps = []string{"VALORANT.exe", "vgc.exe", "RiotClientServices.exe"}

func RuleName(app string) string { return RulePrefix + app }

type Entry struct {
	ID            string                `json:"id"`
	FixID         string                `json:"fixId"`
	At            time.Time             `json:"at"`
	DNS           *platform.DNSState    `json:"dns,omitempty"`
	FirewallRules []string              `json:"firewallRules,omitempty"`
	Service       *platform.ServicePrev `json:"service,omitempty"`
	CrashDump     *platform.RegDWORD    `json:"crashDump,omitempty"`
	HostsBackupID string                `json:"hostsBackupId,omitempty"`
}

var idRe = regexp.MustCompile(`^[0-9a-f]{16}$`)
var backupRe = regexp.MustCompile(`^\d{8}-\d{6}-[0-9a-f]{8}$`)

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func NewBackupID(now time.Time) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return now.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func ValidBackupID(s string) bool { return backupRe.MatchString(s) }

var reversible = map[string]bool{"F04": true, "F06": true, "F07": true, "F08": true, "F09": true, "F10": true}

func validIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil && strings.Count(s, ".") == 3
}

func Validate(e Entry) error {
	if !idRe.MatchString(e.ID) {
		return errors.New("bad id")
	}
	if !reversible[e.FixID] {
		return errors.New("fix not reversible")
	}
	payloads := 0
	if e.DNS != nil {
		payloads++
		if e.FixID != "F04" || e.DNS.LUID == 0 || len(e.DNS.Servers) > 4 {
			return errors.New("bad dns")
		}
		for _, s := range e.DNS.Servers {
			if !validIPv4(s) {
				return errors.New("bad dns server")
			}
		}
	}
	if len(e.FirewallRules) > 0 {
		payloads++
		if e.FixID != "F06" || len(e.FirewallRules) > len(AllowedRuleApps) {
			return errors.New("bad rules")
		}
		for _, r := range e.FirewallRules {
			ok := false
			for _, a := range AllowedRuleApps {
				if r == RuleName(a) {
					ok = true
				}
			}
			if !ok {
				return errors.New("foreign rule")
			}
		}
	}
	if e.Service != nil {
		payloads++
		if (e.FixID != "F07" && e.FixID != "F09") || !platform.ValidStartType(e.Service.StartType) {
			return errors.New("bad service")
		}
		if e.FixID == "F07" && e.Service.Name != "vgc" {
			return errors.New("bad service name")
		}
		if e.FixID == "F09" && e.Service.Name != "" {
			return errors.New("service name is rediscovered by worker")
		}
	}
	if e.CrashDump != nil {
		payloads++
		if e.FixID != "F08" || e.CrashDump.Value > 7 {
			return errors.New("bad crash dump")
		}
	}
	if e.HostsBackupID != "" {
		payloads++
		if e.FixID != "F10" || !ValidBackupID(e.HostsBackupID) {
			return errors.New("bad backup id")
		}
	}
	if payloads != 1 {
		return errors.New("entry must carry exactly one change")
	}
	return nil
}

type Journal struct {
	mu      sync.Mutex
	path    string
	Entries []Entry `json:"entries"`
}

func Open(path string) (*Journal, error) {
	j := &Journal{path: path, Entries: []Entry{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return j, nil
	}
	if err != nil {
		return j, err
	}
	var data struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return j, err
	}
	for _, e := range data.Entries {
		if Validate(e) == nil {
			j.Entries = append(j.Entries, e)
		}
	}
	return j, nil
}

func (j *Journal) List() []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]Entry{}, j.Entries...)
}

func (j *Journal) Add(entries ...Entry) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Entries = append(j.Entries, entries...)
	return j.saveLocked()
}

func (j *Journal) Remove(ids []string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	del := map[string]bool{}
	for _, id := range ids {
		del[id] = true
	}
	kept := []Entry{}
	for _, e := range j.Entries {
		if !del[e.ID] {
			kept = append(kept, e)
		}
	}
	j.Entries = kept
	return j.saveLocked()
}

func (j *Journal) saveLocked() error {
	b, err := json.MarshalIndent(struct {
		Entries []Entry `json:"entries"`
	}{j.Entries}, "", "  ")
	if err != nil {
		return err
	}
	return WriteAtomic(j.path, b)
}

func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
