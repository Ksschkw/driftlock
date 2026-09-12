package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// auditTimeout bounds the on-chain submission. Auditing is an optional extra,
// and an unreachable RPC endpoint must never stall a commit.
const auditTimeout = 30 * time.Second

// SendSolanaAudit records the given hash on the Solana blockchain using the
// built-in Memo program. The hash is stored as the memo data.
//
// The audit is best-effort by design: the caller logs a failure and lets the
// commit continue, because a blockchain outage says nothing about whether the
// documentation is correct.
func SendSolanaAudit(ctx context.Context, cfg config.AuditConfig, hash string) error {
	if !cfg.Solana {
		return nil
	}
	if cfg.RPCEndpoint == "" {
		return fmt.Errorf("solana audit is enabled but audit.rpc_endpoint is empty")
	}
	if cfg.ProgramID != "" {
		// Refuse clearly rather than pretending. A generic "write this hash"
		// call cannot be constructed for an arbitrary program: the instruction
		// layout is program-specific.
		return fmt.Errorf("solana audit: audit.program_id is set to %q, but only the built-in Memo program is supported; clear program_id to use it", cfg.ProgramID)
	}

	ctx, cancel := context.WithTimeout(ctx, auditTimeout)
	defer cancel()
	return sendMemoTransaction(ctx, cfg, hash)
}

// expandHome resolves a leading "~" to the user's home directory. The
// documented default, `~/.config/solana/id.json`, otherwise reached os.ReadFile
// verbatim and failed with "no such file or directory".
func expandHome(path string) (string, error) {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path, nil
	}
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		// A "~user" form is not supported; leave it untouched so the error
		// names the path the user wrote.
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot expand %q: %w", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func sendMemoTransaction(ctx context.Context, cfg config.AuditConfig, hash string) error {
	keypairPath, err := expandHome(cfg.KeypairPath)
	if err != nil {
		return err
	}
	if keypairPath == "" {
		return fmt.Errorf("solana audit is enabled but audit.keypair_path is empty")
	}

	keypairBytes, err := os.ReadFile(keypairPath)
	if err != nil {
		return fmt.Errorf("failed to read solana keypair %s: %w", keypairPath, err)
	}
	keypair, err := solana.PrivateKeyFromBase58(strings.TrimSpace(string(keypairBytes)))
	if err != nil {
		return fmt.Errorf("invalid keypair %s: %w", keypairPath, err)
	}

	client := rpc.New(cfg.RPCEndpoint)

	instruction := solana.NewInstruction(
		solana.MemoProgramID,
		[]*solana.AccountMeta{
			{PublicKey: keypair.PublicKey(), IsSigner: true, IsWritable: false},
		},
		[]byte(hash),
	)

	recent, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return fmt.Errorf("failed to get recent blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recent.Value.Blockhash,
		solana.TransactionPayer(keypair.PublicKey()),
	)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(keypair.PublicKey()) {
			return &keypair
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	opts := rpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: rpc.CommitmentFinalized,
	}
	sig, err := client.SendTransactionWithOpts(ctx, tx, opts)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %w", err)
	}
	fmt.Printf("Solana audit log submitted, tx: %s\n", sig)
	return nil
}
