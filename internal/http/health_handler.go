package http

import (
	"time"

	"log/slog"

	"github.com/karloscodes/cartridge"
)

// HealthStatus represents the health check response
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	DBStatus  string    `json:"db_status"`
}

// HealthIndexAction handles the health check endpoint
func HealthIndexAction(ctx *cartridge.Context) error {
	dbStatus := "ok"

	// Check database connectivity
	db := ctx.DBManager.GetConnection()
	if db == nil {
		dbStatus = "error"
		ctx.Logger.Error("Database connection unavailable")
	} else {
		sqlDB, err := db.DB()
		if err != nil {
			dbStatus = "error"
			ctx.Logger.Error("Database connection error", slog.Any("error", err))
		} else if err := sqlDB.Ping(); err != nil {
			dbStatus = "error"
			ctx.Logger.Error("Database ping failed", slog.Any("error", err))
		}
	}

	health := HealthStatus{
		Status:    "ok",
		Timestamp: time.Now(),
		DBStatus:  dbStatus,
	}

	if dbStatus != "ok" {
		health.Status = "degraded"
	}

	return ctx.JSON(health)
}
