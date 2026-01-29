package database

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds GORM database configuration.
type Config struct {
	DSN             string        // Database DSN
	MaxOpenConns    int           // Maximum open connections (default: 25)
	MaxIdleConns    int           // Maximum idle connections (default: 5)
	ConnMaxLifetime time.Duration // Connection max lifetime (default: 5 minutes)
	ConnMaxIdleTime time.Duration // Connection max idle time (default: 10 minutes)
	LogLevel        logger.LogLevel // Log level (default: Silent, use zap for logging)
	Logger          *zap.Logger   // Optional zap logger for GORM
}

// NewGORM creates a new GORM database connection with optimized settings.
func NewGORM(cfg Config) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("DSN is required")
	}

	// Set defaults
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 5 * time.Minute
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = 10 * time.Minute
	}
	if cfg.LogLevel == 0 {
		cfg.LogLevel = logger.Silent // Use zap logger instead
	}

	// Configure GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(cfg.LogLevel),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
		PrepareStmt: true, // Enable prepared statement cache
	}

	// Open database connection
	db, err := gorm.Open(mysql.Open(cfg.DSN), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// WithContext returns a GORM DB instance with context.
// This is a helper function to ensure consistent context usage.
func WithContext(db *gorm.DB, ctx interface{}) *gorm.DB {
	if ctx == nil {
		return db
	}
	// Try to use WithContext if context.Context is available
	if ctxDB, ok := ctx.(*gorm.DB); ok {
		return ctxDB
	}
	// Fallback to direct db access
	return db
}

// Transaction executes a function within a database transaction.
// It automatically handles rollback on error and panic recovery.
func Transaction(db *gorm.DB, fn func(*gorm.DB) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // Re-panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// TransactionWithContext executes a function within a database transaction with context.
// ctx can be context.Context or *gorm.DB
func TransactionWithContext(db *gorm.DB, ctx interface{}, fn func(*gorm.DB) error) error {
	var tx *gorm.DB
	
	// If ctx is context.Context, use WithContext
	if ctxCtx, ok := ctx.(context.Context); ok {
		tx = db.WithContext(ctxCtx).Begin()
	} else if ctxDB, ok := ctx.(*gorm.DB); ok {
		// If ctx is already a *gorm.DB (with context), use it
		tx = ctxDB.Begin()
	} else {
		// Otherwise, use the base db
		tx = db.Begin()
	}

	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

