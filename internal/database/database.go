package database

import (
	"log/slog"

	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/ai"
	"fusionaly/internal/analytics"
	"fusionaly/internal/annotations"
	"fusionaly/internal/config"
	"fusionaly/internal/events"
	"fusionaly/internal/feed"
	"fusionaly/internal/onboarding"
	"github.com/karloscodes/cartridge/cache"
	"fusionaly/internal/settings"
	"fusionaly/internal/users"
	"fusionaly/internal/websites"
)

// DBManager wraps cartridge's sqlite.Manager with fusionaly-specific migration methods.
type DBManager struct {
	*sqlite.Manager
	logger *slog.Logger
}

// NewDBManager creates a new database manager using cartridge's sqlite.Manager.
func NewDBManager(cfg *config.Config, logger *slog.Logger) *DBManager {
	sqliteCfg := sqlite.Config{
		Path:         cfg.DatabaseName,
		MaxOpenConns: cfg.GetMaxOpenConns(),
		MaxIdleConns: cfg.GetMaxIdleConns(),
		Logger:       logger,
		EnableWAL:    true,
		TxImmediate:  true,
		BusyTimeout:  5000,
	}

	return &DBManager{
		Manager: sqlite.NewManager(sqliteCfg),
		logger:  logger,
	}
}

// Init initializes the database connection.
func (dm *DBManager) Init() error {
	_, err := dm.Manager.Connect()
	return err
}

// MigrateDatabase runs fusionaly-specific migrations.
func (dm *DBManager) MigrateDatabase() error {
	db := dm.GetConnection()
	if db == nil {
		return gorm.ErrInvalidDB
	}

	// Run migrations in a transaction
	err := db.Transaction(func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&cache.CacheRecord{},
			&events.Event{},
			&events.IngestedEvent{},
			&users.User{},
			&settings.Setting{},
			&websites.Website{},
			&analytics.SiteStat{},
			&analytics.PageStat{},
			&analytics.RefStat{},
			&analytics.BrowserStat{},
			&analytics.OSStat{},
			&analytics.DeviceStat{},
			&analytics.CountryStat{},
			&analytics.UTMStat{},
			&analytics.EventStat{},
			&analytics.QueryParamStat{},
			&analytics.FlowTransitionStat{},
			&onboarding.OnboardingSession{},
			&annotations.Annotation{},
			&feed.FeedItem{},
			&feed.FeedBaseline{},
			&ai.SavedQuery{},
			&ai.AIQueryCache{},
		)
	})
	if err != nil {
		dm.logger.Error("Failed to auto-migrate database", slog.Any("error", err))
		return err
	}

	// One-time migration for installs upgrading from Fusionaly Pro: copy the
	// OpenAI API key out of the legacy pro_settings table into OSS settings.
	// Idempotent and a no-op on fresh installs.
	if err := MigrateProSettings(db); err != nil {
		dm.logger.Error("Failed to migrate Pro settings", slog.Any("error", err))
		return err
	}

	// One-time cleanup of legacy low-volume traffic_drop feed items left over from
	// before the drop detector was retuned (see feed.CleanupLegacyDrops). Safe to
	// run on every boot; it only ever deletes drops the current rule won't produce.
	if err := feed.CleanupLegacyDrops(db); err != nil {
		dm.logger.Error("Failed to clean up legacy feed drops", slog.Any("error", err))
		return err
	}

	if err := dm.CheckpointWAL("FULL"); err != nil {
		dm.logger.Warn("Failed to checkpoint WAL after migration", slog.Any("error", err))
	}

	dm.logger.Info("Database migration completed successfully")
	return nil
}
