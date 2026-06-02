package audit

import (
	"context"
	"fmt"
	"os"

	"github.com/Ksschkw/driftlock/internal/config"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// SendSolanaAudit records the given hash on the Solana blockchain using the
// built-in Memo program. The hash is stored as the memo data.
// This provides an immutable, publicly verifiable audit trail.
func SendSolanaAudit(ctx context.Context, cfg config.AuditConfig, hash string) error {
	if !cfg.Solana {
		return nil
	}
	if cfg.RPCEndpoint == "" {
		return fmt.Errorf("solana rpc endpoint not configured")
	}
	if cfg.ProgramID != "" {
		return sendToCustomProgram(ctx, cfg, hash)
	}

	return sendMemoTransaction(ctx, cfg, hash)
}

func sendMemoTransaction(ctx context.Context, cfg config.AuditConfig, hash string) error {
	keypairBytes, err := os.ReadFile(cfg.KeypairPath)
	if err != nil {
		return fmt.Errorf("failed to read solana keypair: %w", err)
	}
	keypair, err := solana.PrivateKeyFromBase58(string(keypairBytes))
	if err != nil {
		return fmt.Errorf("invalid keypair: %w", err)
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

func sendToCustomProgram(ctx context.Context, cfg config.AuditConfig, hash string) error {
	return fmt.Errorf("custom program support not yet implemented; remove program_id to use the default Memo program")
}