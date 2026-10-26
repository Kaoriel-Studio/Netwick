package services

import (
	"strings"
	"sync"
	"time"

	"github.com/kaoriel-studio/netwick/internal/verdict"
)

const EventNavigate = "app:navigate"

const ClientProcess = "riotclientservices.exe"

type Watcher struct {
	st       *State
	notify   func(issue bool)
	mu       sync.Mutex
	stop     chan struct{}
	interval time.Duration
}

func NewWatcher(st *State, notify func(issue bool)) *Watcher {
	return &Watcher{st: st, notify: notify, interval: 5 * time.Second}
}

func (w *Watcher) Set(on bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !on {
		if w.stop != nil {
			close(w.stop)
			w.stop = nil
		}
		return
	}
	if w.stop != nil {
		return
	}
	w.stop = make(chan struct{})
	go w.loop(w.stop)
}

func (w *Watcher) clientRunning() bool {
	procs, err := w.st.Env.Sys.Processes()
	if err != nil {
		return false
	}
	for _, p := range procs {
		if strings.EqualFold(p.Name, ClientProcess) {
			return true
		}
	}
	return false
}

func (w *Watcher) loop(stop chan struct{}) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	was := w.clientRunning()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			now := w.clientRunning()
			if now && !was {
				sum, err := w.st.run(ModeQuick, "", nil)
				if err == nil && sum.Verdict.Level != verdict.Green && w.notify != nil {
					w.notify(true)
				}
			}
			was = now
		}
	}
}
