package probe

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	StatusOk        Status = "Ok"
	StatusStalled   Status = "Stalled"
	StatusCertError Status = "CertError"
	StatusEmpty     Status = "Empty"
	StatusNoTLS     Status = "NoTls"
	StatusNoTCP     Status = "NoTcp"
)

type Cause string

const (
	CauseNone    Cause = ""
	CauseTimeout Cause = "timeout"
	CauseReset   Cause = "reset"
)

type Config struct {
	Port         int
	StageTimeout time.Duration
	MaxBytes     int
	UserAgent    string
	RootCAs      *x509.CertPool
	Dialer       *net.Dialer
}

func DefaultConfig(userAgent string) Config {
	return Config{
		Port:         443,
		StageTimeout: 4 * time.Second,
		MaxBytes:     64 * 1024,
		UserAgent:    userAgent,
	}
}

type Result struct {
	IP         string    `json:"ip"`
	Host       string    `json:"host"`
	Status     Status    `json:"status"`
	Cause      Cause     `json:"cause"`
	CertError  string    `json:"certError"`
	LatencyMs  int       `json:"latencyMs"`
	Bytes      int       `json:"bytes"`
	HTTPStatus int       `json:"httpStatus"`
	ServerDate time.Time `json:"serverDate"`
	MeasuredAt time.Time `json:"measuredAt"`
	Pinned     bool      `json:"pinned"`
}

func (r Result) OK() bool { return r.Status == StatusOk }

func Probe(ctx context.Context, ip, host string, cfg Config) Result {
	res := Result{IP: ip, Host: host, Status: StatusNoTCP}
	if net.ParseIP(ip) == nil {
		res.Cause = CauseReset
		return res
	}
	if cfg.StageTimeout <= 0 {
		cfg.StageTimeout = 4 * time.Second
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 64 * 1024
	}
	if cfg.Port == 0 {
		cfg.Port = 443
	}
	dialer := cfg.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	d := *dialer
	d.Timeout = cfg.StageTimeout

	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(cfg.Port)))
	if err != nil {
		res.Cause = classify(err, time.Since(start), cfg.StageTimeout)
		return res
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	res.Status = StatusNoTLS
	tlsCfg := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
		RootCAs:    cfg.RootCAs,
	}
	tlsStart := time.Now()
	_ = conn.SetDeadline(tlsStart.Add(cfg.StageTimeout))
	tc := tls.Client(conn, tlsCfg)
	if err := tc.HandshakeContext(ctx); err != nil {
		var cve *tls.CertificateVerificationError
		if errors.As(err, &cve) {
			res.Status = StatusCertError
			res.CertError = shortCertError(cve.Err)
			res.LatencyMs = int(time.Since(start).Milliseconds())
			return res
		}
		elapsed := time.Since(tlsStart)
		if elapsed >= cfg.StageTimeout-250*time.Millisecond || isTimeout(err) {
			res.Cause = CauseTimeout
		} else {
			res.Cause = CauseReset
		}
		return res
	}
	res.LatencyMs = int(time.Since(start).Milliseconds())

	res.Status = StatusStalled
	req := "GET / HTTP/1.1\r\nHost: " + host + "\r\nUser-Agent: " + cfg.UserAgent + "\r\nAccept: */*\r\nConnection: close\r\n\r\n"
	_ = tc.SetDeadline(time.Now().Add(cfg.StageTimeout))
	if _, err := tc.Write([]byte(req)); err != nil {
		res.Cause = classify(err, 0, cfg.StageTimeout)
		return res
	}

	complete, n, cause, head := readResponse(tc, cfg)
	res.Bytes = n
	if head != nil {
		res.HTTPStatus = head.status
		res.ServerDate = head.date
		if !head.date.IsZero() {
			res.MeasuredAt = head.at
		}
	}
	if !complete {
		res.Cause = cause
		return res
	}
	if n == 0 {
		res.Status = StatusEmpty
	} else {
		res.Status = StatusOk
	}
	return res
}

type header struct {
	status        int
	contentLength int64
	chunked       bool
	date          time.Time
	at            time.Time
	end           int
}

func readResponse(c net.Conn, cfg Config) (bool, int, Cause, *header) {
	buf := make([]byte, 8192)
	var head bytes.Buffer
	var hdr *header
	total := 0
	tail := make([]byte, 0, 5)
	for {
		_ = c.SetReadDeadline(time.Now().Add(cfg.StageTimeout))
		n, err := c.Read(buf)
		if n > 0 {
			total += n
			if hdr == nil && head.Len() < cfg.MaxBytes {
				head.Write(buf[:n])
				if idx := bytes.Index(head.Bytes(), []byte("\r\n\r\n")); idx >= 0 {
					hdr = parseHeader(head.Bytes()[:idx])
					hdr.end = idx + 4
					hdr.at = time.Now().UTC()
				}
			}
			tail = append(tail, buf[:n]...)
			if len(tail) > 5 {
				tail = tail[len(tail)-5:]
			}
			if hdr != nil && hdr.contentLength >= 0 && int64(total-hdr.end) >= hdr.contentLength {
				return true, total, CauseNone, hdr
			}
			if hdr != nil && hdr.chunked && bytes.Equal(tail, []byte("0\r\n\r\n")) {
				return true, total, CauseNone, hdr
			}
			if total >= cfg.MaxBytes {
				return true, total, CauseNone, hdr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return true, total, CauseNone, hdr
			}
			return false, total, classify(err, 0, cfg.StageTimeout), hdr
		}
	}
}

func parseHeader(b []byte) *header {
	h := &header{contentLength: -1}
	lines := strings.Split(string(b), "\r\n")
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) >= 2 {
			if code, err := strconv.Atoi(parts[1]); err == nil {
				h.status = code
			}
		}
	}
	for _, line := range lines[1:] {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "content-length":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
				h.contentLength = n
			}
		case "transfer-encoding":
			if strings.Contains(strings.ToLower(v), "chunked") {
				h.chunked = true
			}
		case "date":
			if t, err := http.ParseTime(v); err == nil {
				h.date = t.UTC()
			} else if t, err := time.Parse(time.RFC1123, v); err == nil {
				h.date = t.UTC()
			}
		}
	}
	return h
}

func shortCertError(err error) string {
	var unk x509.UnknownAuthorityError
	var host x509.HostnameError
	var inv x509.CertificateInvalidError
	switch {
	case errors.As(err, &unk):
		return "untrusted"
	case errors.As(err, &host):
		return "hostname"
	case errors.As(err, &inv):
		if inv.Reason == x509.Expired {
			return "expired"
		}
		return "invalid"
	default:
		return "invalid"
	}
}

func isTimeout(err error) bool {
	if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func classify(err error, elapsed, timeout time.Duration) Cause {
	if isTimeout(err) {
		return CauseTimeout
	}
	if timeout > 0 && elapsed >= timeout-250*time.Millisecond {
		return CauseTimeout
	}
	return CauseReset
}

func (r Result) String() string {
	return fmt.Sprintf("%s %s %s %s", r.Host, r.IP, r.Status, r.Cause)
}
