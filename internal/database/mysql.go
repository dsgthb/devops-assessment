package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Config holds database configuration
type Config struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// DB holds the database connection pool
type DB struct {
	*sql.DB
}

// NewConnection creates a new database connection
func NewConnection(cfg Config) (*DB, error) {
	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	// Open database connection
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		sqlDB.SetMaxOpenConns(25)
	}

	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		sqlDB.SetMaxIdleConns(5)
	}

	if cfg.MaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)
	} else {
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	// Test connection
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{sqlDB}

	// Run migrations
	if err := RunMigrations(sqlDB); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database connection established successfully")
	return db, nil
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			// Rollback on panic
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// QueryRowContext executes a query that returns at most one row with context
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.DB.QueryRowContext(ctx, query, args...)
}

// QueryContext executes a query that returns rows with context
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.DB.QueryContext(ctx, query, args...)
}

// ExecContext executes a query without returning any rows with context
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.DB.ExecContext(ctx, query, args...)
}

// PrepareContext creates a prepared statement with context
func (db *DB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return db.DB.PrepareContext(ctx, query)
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// Helper functions for common database operations

// GetOne executes a query and scans the result into dest
func (db *DB) GetOne(dest interface{}, query string, args ...interface{}) error {
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	row := db.QueryRowContext(ctx, query, args...)
	if err := row.Scan(dest); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no rows found")
		}
		return fmt.Errorf("failed to scan row: %w", err)
	}
	return nil
}

// GetMany executes a query and returns multiple results
func (db *DB) GetMany(query string, args ...interface{}) (*sql.Rows, error) {
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	return rows, nil
}

// Insert executes an insert query and returns the last insert ID
func (db *DB) Insert(query string, args ...interface{}) (int64, error) {
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// Update executes an update query and returns the number of affected rows
func (db *DB) Update(query string, args ...interface{}) (int64, error) {
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute update: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return affected, nil
}

// Delete executes a delete query and returns the number of affected rows
func (db *DB) Delete(query string, args ...interface{}) (int64, error) {
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute delete: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return affected, nil
}

// Exists checks if a record exists
func (db *DB) Exists(query string, args ...interface{}) (bool, error) {
	var exists bool
	query = fmt.Sprintf("SELECT EXISTS(%s)", query)
	
	ctx, cancel := newContextWithTimeout()
	defer cancel()

	err := db.QueryRowContext(ctx, query, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return exists, nil
}

// Context helpers

// newContextWithTimeout creates a new context with default timeout
func newContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// newContextWithCustomTimeout creates a new context with custom timeout
func newContextWithCustomTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}