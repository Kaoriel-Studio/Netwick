//go:build windows

package ipc

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/kaoriel-studio/netwick/internal/app"
	"github.com/kaoriel-studio/netwick/internal/worker"
	"golang.org/x/sys/windows"
)

const PipePrefix = `\\.\pipe\netwick-`

var (
	modShell32         = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteEx = modShell32.NewProc("ShellExecuteExW")
)

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	seeMaskFlagNoUI       = 0x00000400
	swHide                = 0
)

func elevate(exe string, args []string) (windows.Handle, error) {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	params, err := windows.UTF16PtrFromString(strings.Join(args, " "))
	if err != nil {
		return 0, err
	}
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess | seeMaskNoAsync,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		lpDirectory:  dir,
		nShow:        swHide,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	r, _, e := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		if errors.Is(e, windows.ERROR_CANCELLED) {
			return 0, app.ErrDeclined
		}
		return 0, e
	}
	if info.hProcess == 0 {
		return 0, errors.New("no process handle")
	}
	return info.hProcess, nil
}

func safeArg(a string) bool {
	return a != "" && !strings.ContainsAny(a, " \t\"&|<>^%\r\n")
}

type ElevatedRunner struct{}

func (ElevatedRunner) Run(ctx context.Context, a worker.Args, req worker.Request) (worker.Response, error) {
	var resp worker.Response
	if _, err := worker.ParseArgs(a.Build()); err != nil {
		return resp, err
	}
	args := a.Build()
	for _, x := range args {
		if !safeArg(x) {
			return resp, errors.New("unsafe argument")
		}
	}
	sid, err := currentUserSID()
	if err != nil {
		return resp, err
	}
	l, err := winio.ListenPipe(PipePrefix+a.Pipe, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + sid + ")(A;;GA;;;BA)",
		InputBufferSize:    MaxMessage,
		OutputBufferSize:   MaxMessage,
	})
	if err != nil {
		return resp, err
	}
	defer l.Close()

	exe, err := os.Executable()
	if err != nil {
		return resp, err
	}
	proc, err := elevate(exe, args)
	if err != nil {
		return resp, err
	}
	defer windows.CloseHandle(proc)
	pid, err := windows.GetProcessId(proc)
	if err != nil {
		return resp, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	go func() {
		<-ctx.Done()
		l.Close()
	}()
	exited := make(chan struct{})
	go func() {
		_, _ = windows.WaitForSingleObject(proc, windows.INFINITE)
		close(exited)
		time.Sleep(500 * time.Millisecond)
		l.Close()
	}()

	conn, err := l.Accept()
	if err != nil {
		return resp, errors.New("worker did not connect")
	}
	defer conn.Close()
	l.Close()
	if err := verifyClient(conn, pid); err != nil {
		return resp, err
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Minute))
	if err := Write(conn, req); err != nil {
		return resp, err
	}
	if err := Read(conn, &resp); err != nil {
		return resp, err
	}
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		_ = windows.TerminateProcess(proc, 1)
	}
	return resp, nil
}

type fdConn interface{ Fd() uintptr }

func verifyClient(c net.Conn, want uint32) error {
	fc, ok := c.(fdConn)
	if !ok {
		return errors.New("cannot verify pipe client")
	}
	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(fc.Fd()), &pid); err != nil {
		return err
	}
	if pid != want {
		return errors.New("unexpected pipe client")
	}
	return nil
}

func currentUserSID() (string, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

func Dial(pipe string) (net.Conn, error) {
	timeout := 10 * time.Second
	c, err := winio.DialPipe(PipePrefix+pipe, &timeout)
	if err != nil {
		return nil, err
	}
	if err := verifyServer(c); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func verifyServer(c net.Conn) error {
	fc, ok := c.(fdConn)
	if !ok {
		return errors.New("cannot verify pipe server")
	}
	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(windows.Handle(fc.Fd()), &pid); err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return err
	}
	server := windows.UTF16ToString(buf[:n])
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(server), filepath.Clean(self)) {
		return errors.New("pipe server is not Netwick")
	}
	return nil
}
