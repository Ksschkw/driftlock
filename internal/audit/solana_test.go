package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ksschkw/driftlock/internal/config"
)

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cases := map[string]string{
		"":                         "",
		"/abs/path":                "/abs/path",
		"relative/path":            "relative/path",
		"~":                        home,
		"~/.config/solana/id.json": filepath.Join(home, ".config", "solana", "id.json"),
		"~other/path":              "~other/path", // unsupported form, left as written
	}
	for in, want := range cases {
		got, err := expandHome(in)
		if err != nil {
			t.Fatalf("expandHome(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

// The documented default keypair path starts with "~"; it must resolve, or the
// audit fails with a confusing "no such file" for a path the user copied from
// the documentation.
func TestSendSolanaAuditExpandsHomeInKeypairPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "solana")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Invalid keypair content: the error must be about parsing the file at the
	// EXPANDED path, proving the path was resolved.
	if err := os.WriteFile(filepath.Join(dir, "id.json"), []byte("not-a-key"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := SendSolanaAudit(context.Background(), config.AuditConfig{
		Solana:      true,
		RPCEndpoint: "http://127.0.0.1:1",
		KeypairPath: "~/.config/solana/id.json",
	}, "hash")
	if err == nil {
		t.Fatal("expected an error for an invalid keypair")
	}
	if !strings.Contains(err.Error(), filepath.Join(home, ".config", "solana", "id.json")) {
		t.Errorf("error does not name the expanded path: %v", err)
	}
}

func TestSendSolanaAuditDisabledIsNoOp(t *testing.T) {
	if err := SendSolanaAudit(context.Background(), config.AuditConfig{Solana: false}, "hash"); err != nil {
		t.Fatalf("disabled audit returned an error: %v", err)
	}
}

func TestSendSolanaAuditRequiresRPCEndpoint(t *testing.T) {
	err := SendSolanaAudit(context.Background(), config.AuditConfig{Solana: true}, "hash")
	if err == nil || !strings.Contains(err.Error(), "rpc_endpoint") {
		t.Fatalf("expected an rpc_endpoint error, got %v", err)
	}
}

// program_id is documented but cannot be honoured, because a generic hash
// submission needs a program-specific instruction layout. It must fail with an
// explanation rather than a vague stub error.
func TestSendSolanaAuditCustomProgramIsClearlyUnsupported(t *testing.T) {
	err := SendSolanaAudit(context.Background(), config.AuditConfig{
		Solana:      true,
		RPCEndpoint: "http://127.0.0.1:1",
		ProgramID:   "Memo1111111111111111111111111111111111111",
	}, "hash")
	if err == nil {
		t.Fatal("expected an error for a custom program")
	}
	if !strings.Contains(err.Error(), "only the built-in Memo program is supported") {
		t.Errorf("error does not explain the limitation: %v", err)
	}
	if !strings.Contains(err.Error(), "clear program_id") {
		t.Errorf("error does not say how to fix it: %v", err)
	}
}

func TestSendSolanaAuditRequiresKeypairPath(t *testing.T) {
	err := SendSolanaAudit(context.Background(), config.AuditConfig{
		Solana:      true,
		RPCEndpoint: "http://127.0.0.1:1",
	}, "hash")
	if err == nil || !strings.Contains(err.Error(), "keypair_path") {
		t.Fatalf("expected a keypair_path error, got %v", err)
	}
}
