package config

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DBConfig holds common database connection settings.
type DBConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
}

// DefaultDBConfig returns sensible defaults for a typical service.
func DefaultDBConfig() DBConfig {
	return DBConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		PingTimeout:     5 * time.Second,
	}
}

// OpenDB opens a database connection and applies connection settings.
// It performs a ping to verify the connection before returning.
func OpenDB(ctx context.Context, driver, dsn string, cfg DBConfig) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, fmt.Errorf("%w; close error: %v", err, cerr)
		}
		return nil, err
	}

	return db, nil
}

// WithTransaction runs fn inside a transaction and commits or rolls back.
func WithTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			if rerr := tx.Rollback(); rerr != nil {
				panic(fmt.Errorf("panic: %v; rollback error: %w", p, rerr))
			}
			panic(p)
		}

		if err != nil {
			if rerr := tx.Rollback(); rerr != nil {
				err = fmt.Errorf("%v; rollback error: %w", err, rerr)
			}
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CloseDB closes the database connection gracefully.
func CloseDB(db *sql.DB) error {
	return db.Close()
}
