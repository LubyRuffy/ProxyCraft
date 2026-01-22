package proxy

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "proxycraft-certs-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create temp cert dir:", err)
		os.Exit(1)
	}
	if err := os.Setenv("PROXYCRAFT_CERT_DIR", tmpDir); err != nil {
		fmt.Fprintln(os.Stderr, "failed to set PROXYCRAFT_CERT_DIR:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}
