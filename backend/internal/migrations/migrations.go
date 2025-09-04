package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run executa migrações mínimas para Client
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clients (
            id TEXT PRIMARY KEY,
            secret_id TEXT UNIQUE NOT NULL,
            name TEXT NOT NULL,
            webhook_url TEXT NOT NULL,
            plan TEXT NOT NULL DEFAULT 'FREE',
            rate_limit_per_min INT NOT NULL DEFAULT 60,
            is_active BOOLEAN NOT NULL DEFAULT TRUE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,
		`CREATE INDEX IF NOT EXISTS idx_clients_secret_id ON clients(secret_id);`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
