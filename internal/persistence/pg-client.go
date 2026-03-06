package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGClient struct {
	pool *pgxpool.Pool
}

// NewPGClient creates a pg connection pool using pg url
func NewPGClient(pgURL string) (*PGClient, error) {
	pgCtx, pgCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pgCancel()
	pool, pgErr := pgxpool.New(pgCtx, pgURL)
	if pgErr != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL pool: %w", pgErr)
	}
	if pgErr = pool.Ping(pgCtx); pgErr != nil {
		return nil, fmt.Errorf("failed to reach PostgreSQL: %w", pgErr)
	}

	return &PGClient{pool}, nil
}

// GetPool return pg pool instance
func (pgc *PGClient) GetPool() *pgxpool.Pool {
	return pgc.pool
}

// CloseConnection close pg connection
func (pgc *PGClient) CloseConnection() {
	if pgc.pool != nil {
		pgc.pool.Close()
		pgc.pool = nil
	}
}
