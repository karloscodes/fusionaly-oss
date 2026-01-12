package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"

	"log/slog"

	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"
)

// JSON is a custom type for handling JSON data
type JSON []byte

// Scan scan value into Jsonb, implements sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := json.RawMessage{}
	err := json.Unmarshal(bytes, &result)
	*j = JSON(result)
	return err
}

// Value return json value, implement driver.Valuer interface
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.RawMessage(j).MarshalJSON()
}

// MarshalJSON implements the json.Marshaler interface
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return fmt.Errorf("JSON: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

// PerformWrite executes a write transaction with retry logic for SQLite busy errors.
// This is a wrapper that delegates to cartridge's sqlite.PerformWrite implementation.
func PerformWrite(logger *slog.Logger, dbConn *gorm.DB, f func(tx *gorm.DB) error) error {
	return sqlite.PerformWrite(logger, dbConn, f)
}
