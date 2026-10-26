package probe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

const testHost = "probe.riotgames.test"

func makeCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: testHost},
		DNSNames:              []string{testHost},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

type behavior func(c net.Conn, cert tls.Certificate)

func serve(t *testing.T, cert tls.Certificate, b behavior) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go b(c, cert)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

func tlsServe(handler func(tc *tls.Conn)) behavior {
	return func(c net.Conn, cert tls.Certificate) {
		tc := tls.Server(c, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
		defer tc.Close()
		if err := tc.Handshake(); err != nil {
			return
		}
		buf := make([]byte, 4096)
		_, _ = tc.Read(buf)
		handler(tc)
	}
}

func cfgFor(port int, pool *x509.CertPool, timeout time.Duration) Config {
	return Config{Port: port, StageTimeout: timeout, MaxBytes: 64 * 1024, UserAgent: "Netwick/test", RootCAs: pool}
}

func TestTestbed(t *testing.T) {
	cert, pool := makeCert(t)
	big := strings.Repeat("x", 100*1024)
	const timeout = 800 * time.Millisecond

	cases := []struct {
		name      string
		b         behavior
		closed    bool
		untrusted bool
		want      Status
		cause     Cause
	}{
		{name: "small 403 with Content-Length", b: tlsServe(func(tc *tls.Conn) {
			body := "forbidden"
			fmt.Fprintf(tc, "HTTP/1.1 403 Forbidden\r\nDate: %s\r\nContent-Length: %d\r\n\r\n%s", time.Now().UTC().Format(time.RFC1123), len(body), body)
			time.Sleep(2 * timeout)
		}), want: StatusOk},
		{name: "large 100KB response", b: tlsServe(func(tc *tls.Conn) {
			fmt.Fprintf(tc, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n%s", len(big), big)
		}), want: StatusOk},
		{name: "chunked response", b: tlsServe(func(tc *tls.Conn) {
			fmt.Fprint(tc, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n")
			time.Sleep(2 * timeout)
		}), want: StatusOk},
		{name: "serves 16KB and stalls", b: tlsServe(func(tc *tls.Conn) {
			fmt.Fprintf(tc, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n%s", 100*1024, strings.Repeat("y", 16*1024))
			time.Sleep(3 * timeout)
		}), want: StatusStalled, cause: CauseTimeout},
		{name: "TCP accepted, TLS never starts", b: func(c net.Conn, _ tls.Certificate) {
			time.Sleep(3 * timeout)
			c.Close()
		}, want: StatusNoTLS, cause: CauseTimeout},
		{name: "TCP accepted and immediately reset", b: func(c net.Conn, _ tls.Certificate) {
			if tc, ok := c.(*net.TCPConn); ok {
				_ = tc.SetLinger(0)
			}
			c.Close()
		}, want: StatusNoTLS, cause: CauseReset},
		{name: "closed port", closed: true, want: StatusNoTCP, cause: CauseReset},
		{name: "untrusted certificate", untrusted: true, b: tlsServe(func(tc *tls.Conn) {
			fmt.Fprint(tc, "HTTP/1.1 403 Forbidden\r\nContent-Length: 2\r\n\r\nno")
		}), want: StatusCertError},
		{name: "empty response", b: tlsServe(func(tc *tls.Conn) {}), want: StatusEmpty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var port int
			if tc.closed {
				ln, _ := net.Listen("tcp", "127.0.0.1:0")
				port = ln.Addr().(*net.TCPAddr).Port
				ln.Close()
			} else {
				port = serve(t, cert, tc.b)
			}
			roots := pool
			if tc.untrusted {
				roots = x509.NewCertPool()
			}
			res := Probe(context.Background(), "127.0.0.1", testHost, cfgFor(port, roots, timeout))
			if res.Status != tc.want || res.Cause != tc.cause {
				t.Fatalf("got %s/%q, want %s/%q (bytes=%d cert=%s)", res.Status, res.Cause, tc.want, tc.cause, res.Bytes, res.CertError)
			}
		})
	}
}

func TestDateHeaderCaptured(t *testing.T) {
	cert, pool := makeCert(t)
	date := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	port := serve(t, cert, tlsServe(func(tc *tls.Conn) {
		fmt.Fprintf(tc, "HTTP/1.1 403 Forbidden\r\nDate: %s\r\nContent-Length: 0\r\n\r\n", date.Format(time.RFC1123))
	}))
	res := Probe(context.Background(), "127.0.0.1", testHost, cfgFor(port, pool, time.Second))
	if res.Status != StatusOk {
		t.Fatalf("status %s", res.Status)
	}
	if !res.ServerDate.Equal(date) || res.MeasuredAt.IsZero() || res.HTTPStatus != 403 {
		t.Fatalf("date %v measured %v code %d", res.ServerDate, res.MeasuredAt, res.HTTPStatus)
	}
}

func TestConcurrentProbesDoNotPanic(t *testing.T) {
	cert, pool := makeCert(t)
	port := serve(t, cert, tlsServe(func(tc *tls.Conn) {
		fmt.Fprint(tc, "HTTP/1.1 403 Forbidden\r\nContent-Length: 2\r\n\r\nno")
	}))
	done := make(chan Result, 32)
	for i := 0; i < 32; i++ {
		go func() { done <- Probe(context.Background(), "127.0.0.1", testHost, cfgFor(port, pool, time.Second)) }()
	}
	for i := 0; i < 32; i++ {
		if r := <-done; r.Status != StatusOk {
			t.Fatalf("probe %d: %s", i, r.Status)
		}
	}
}

func TestCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := Probe(ctx, "127.0.0.1", testHost, cfgFor(1, nil, time.Second))
	if r.OK() {
		t.Fatal("cancelled probe reported ok")
	}
}

func TestIsRiotHost(t *testing.T) {
	cases := map[string]bool{
		"riotgames.com":                   true,
		"auth.riotgames.com":              true,
		"AUTH.RIOTGAMES.COM.":             true,
		"valorant.secure.dyn.riotcdn.net": true,
		"x.pvp.net":                       true,
		"kr.riotgames.co.kr":              true,
		"notriotgames.com":                false,
		"riotgames.com.evil.com":          false,
		"evil.com":                        false,
		"riotgames.com/evil":              false,
		"":                                false,
		"a1..riotgames.com":               false,
		".riotgames.com":                  false,
	}
	for h, want := range cases {
		if got := IsRiotHost(h); got != want {
			t.Errorf("IsRiotHost(%q) = %v", h, got)
		}
	}
}
