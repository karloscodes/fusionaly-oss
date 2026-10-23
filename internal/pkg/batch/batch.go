// Package batch deletes many rows without holding the database writer long.
package batch

import (
	"context"
	"time"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"
)

// Size is the number of rows that one write deletes.
const Size = 1000

// Delete runs query until it deletes fewer than Size rows, and returns the
// number of rows it deleted. Each run is its own write inside the serialized
// writer, so ingestion waits for one batch at most. GORM drops .Limit() on a
// SQLite DELETE, so query selects its rows with a subquery that ends in
// "LIMIT ?". Delete adds Size as the last argument.
func Delete(dbManager cartridge.DBManager, query string, args ...any) (int64, error) {
	args = append(args, Size)
	var total int64
	for {
		var deleted int64
		err := cartridge.Write(context.Background(), dbManager, func(tx *gorm.DB) error {
			result := tx.Exec(query, args...)
			deleted = result.RowsAffected
			return result.Error
		})
		total += deleted
		if err != nil || deleted < Size {
			return total, err
		}

		// A short pause lets other writes in between batches.
		time.Sleep(100 * time.Millisecond)
	}
}
