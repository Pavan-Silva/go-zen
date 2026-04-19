package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// DB is a wrapper around sql.DB providing convenient query helpers and pooling configuration.
type DB struct {
	conn *sql.DB
	mu   sync.RWMutex
}

// Open opens a database connection with the given driver and data source name.
// Automatically configures sensible connection pool defaults.
func Open(driver, dsn string) (*DB, error) {
	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool with sensible defaults
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Conn returns the underlying *sql.DB for direct access if needed.
func (db *DB) Conn() *sql.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn
}

// SetPool configures connection pool parameters.
func (db *DB) SetPool(maxOpen, maxIdle int, maxLifetime time.Duration) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.conn.SetMaxOpenConns(maxOpen)
	db.conn.SetMaxIdleConns(maxIdle)
	db.conn.SetConnMaxLifetime(maxLifetime)
}

// QueryRow executes a query that returns at most one row.
func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

// Query executes a query that returns multiple rows.
func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

// Exec executes a query without returning rows.
func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// ScanRow scans a single row into the provided destination pointers.
// Returns ErrNoRows if the query matches no rows.
func (db *DB) ScanRow(ctx context.Context, query string, dests []any, args ...any) error {
	row := db.QueryRow(ctx, query, args...)
	return row.Scan(dests...)
}

// ScanAll scans all rows returned by a query into a slice using the provided mapper function.
// The mapper is called for each row and should populate the result from row values.
// Example: mapper := func(r *sql.Rows) (Item, error) { var x Item; r.Scan(&x.ID, &x.Name); return x, nil }
func (db *DB) ScanAll(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.Query(ctx, query, args...)
}

// Tx starts a new transaction.
// The transaction is committed if fn returns nil, otherwise it's rolled back.
func (db *DB) Tx(ctx context.Context, fn func(*Tx) error) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(&Tx{tx: tx}); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// TxOpts starts a new transaction with specific options.
func (db *DB) TxOpts(ctx context.Context, opts *sql.TxOptions, fn func(*Tx) error) error {
	tx, err := db.conn.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(&Tx{tx: tx}); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// Tx represents a database transaction wrapper.
type Tx struct {
	tx *sql.Tx
}

// QueryRow executes a query within a transaction that returns at most one row.
func (t *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

// Query executes a query within a transaction that returns multiple rows.
func (t *Tx) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

// Exec executes a query within a transaction without returning rows.
func (t *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

// ScanRow scans a single row into destination pointers within a transaction.
func (t *Tx) ScanRow(ctx context.Context, query string, dests []any, args ...any) error {
	row := t.QueryRow(ctx, query, args...)
	return row.Scan(dests...)
}

// ScanAll scans all rows returned by a query within a transaction.
func (t *Tx) ScanAll(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Query(ctx, query, args...)
}

// Builder provides SQL query building with parameter binding.
type Builder struct {
	query string
	args  []any
}

// NewBuilder creates a new SQL query builder.
func NewBuilder(query string) *Builder {
	return &Builder{query: query}
}

// Arg appends an argument to the query builder.
func (b *Builder) Arg(arg any) *Builder {
	b.args = append(b.args, arg)
	return b
}

// Args appends multiple arguments to the query builder.
func (b *Builder) Args(args ...any) *Builder {
	b.args = append(b.args, args...)
	return b
}

// Query returns the SQL query string.
func (b *Builder) Query() string {
	return b.query
}

// QueryArgs returns the SQL query string and arguments.
func (b *Builder) QueryArgs() (string, []any) {
	return b.query, b.args
}

// Prepare prepares a statement for repeated execution.
func (db *DB) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	return db.conn.PrepareContext(ctx, query)
}

// IsNotFound returns true if the error is a "no rows" error.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// Nullable represents a nullable database value.
// Use this type to safely handle NULL values in the database.
type Nullable[T any] struct {
	Value T
	Valid bool
}

// NullableInt creates a nullable integer value.
func NullableInt(v int64) Nullable[int64] {
	return Nullable[int64]{Value: v, Valid: true}
}

// NullableString creates a nullable string value.
func NullableString(v string) Nullable[string] {
	return Nullable[string]{Value: v, Valid: true}
}

// NullableFloat creates a nullable float value.
func NullableFloat(v float64) Nullable[float64] {
	return Nullable[float64]{Value: v, Valid: true}
}

// NullableTime creates a nullable time value.
func NullableTime(v time.Time) Nullable[time.Time] {
	return Nullable[time.Time]{Value: v, Valid: true}
}

// Scan implements sql.Scanner for Nullable types.
func (n *Nullable[T]) Scan(value any) error {
	var zero T
	if value == nil {
		n.Value = zero
		n.Valid = false
		return nil
	}

	val := reflect.ValueOf(value)
	typ := reflect.TypeOf(zero)

	if val.Type().AssignableTo(typ) {
		n.Value = value.(T)
		n.Valid = true
		return nil
	}

	if val.Type().ConvertibleTo(typ) {
		converted := val.Convert(typ).Interface()
		n.Value = converted.(T)
		n.Valid = true
		return nil
	}

	return fmt.Errorf("cannot scan %T into %T", value, zero)
}