package database

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const SchemaVersion = 1

type StoreSettings struct {
	ID               bool      `json:"-" gorm:"primaryKey"`
	Name             string    `json:"name"`
	Currency         string    `json:"currency"`
	ShippingFeeMinor int64     `json:"shipping_fee_minor"`
	UpdatedAt        time.Time `json:"-"`
}

func (StoreSettings) TableName() string { return "store_settings" }

func Open(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("database configuration failed")
	}
	pool, err := db.DB()
	if err != nil {
		return nil, errors.New("database pool unavailable")
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)
	if err := pool.PingContext(ctx); err != nil {
		_ = pool.Close()
		return nil, errors.New("database connection failed")
	}
	return db, nil
}

func CheckReady(ctx context.Context, db *gorm.DB) error {
	pool, err := db.DB()
	if err != nil {
		return errors.New("database unavailable")
	}
	if err := pool.PingContext(ctx); err != nil {
		return errors.New("database unavailable")
	}
	var state struct {
		Version int
		Dirty   bool
	}
	result := db.WithContext(ctx).Raw("SELECT version, dirty FROM schema_migrations").Scan(&state)
	if result.Error != nil || result.RowsAffected != 1 || state.Version != SchemaVersion || state.Dirty {
		return errors.New("database schema is not ready")
	}
	_, err = Store(ctx, db)
	return err
}

func Store(ctx context.Context, db *gorm.DB) (StoreSettings, error) {
	var settings StoreSettings
	if err := db.WithContext(ctx).Where("id = ?", true).Take(&settings).Error; err != nil {
		return settings, errors.New("store settings are not ready")
	}
	return settings, nil
}

func Seed(ctx context.Context, db *gorm.DB, name string, shippingFee int64) error {
	if len(strings.TrimSpace(name)) == 0 || utf8.RuneCountInString(name) > 120 || shippingFee < 0 || shippingFee > 9000000000000 {
		return errors.New("invalid seed store settings")
	}
	result := db.WithContext(ctx).Exec("INSERT INTO store_settings (id, name, currency, shipping_fee_minor) VALUES (true, ?, 'VND', ?) ON CONFLICT (id) DO NOTHING", name, shippingFee)
	if result.Error != nil {
		return errors.New("store settings seed failed")
	}
	return nil
}
