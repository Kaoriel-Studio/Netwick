//go:build windows

package ipc

import (
	"testing"
	"unsafe"
)

func TestShellExecuteInfoLayout(t *testing.T) {
	if s := unsafe.Sizeof(shellExecuteInfo{}); s != 112 {
		t.Fatalf("SHELLEXECUTEINFOW size %d, want 112", s)
	}
}

func TestSafeArg(t *testing.T) {
	for _, bad := range []string{"", "a b", "x&calc", "\"q\"", "%PATH%", "a|b"} {
		if safeArg(bad) {
			t.Errorf("accepted %q", bad)
		}
	}
	if !safeArg("--actions=F02,F05") {
		t.Error("rejected valid arg")
	}
}
