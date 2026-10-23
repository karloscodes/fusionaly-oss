package websites

import (
	"fmt"

	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/pkg/batch"
)

// PurgeDeletedWebsites deletes the rows of deleted websites from every table
// with a website_id: raw events, queue rows, summaries, annotations, and feed
// items. It returns the number of rows it deleted. The cleanup job calls it.
// A run that stops halfway is harmless: the next run deletes the rest.
func PurgeDeletedWebsites(dbManager cartridge.DBManager) (int64, error) {
	db := dbManager.GetConnection()
	ids, err := deletedWebsiteIDs(db)
	if err != nil || len(ids) == 0 {
		return 0, err
	}

	// Read the tables from the schema, so a new table is never forgotten.
	var tables []string
	err = db.Raw(`SELECT m.name FROM sqlite_master m, pragma_table_info(m.name) p
		WHERE m.type = 'table' AND p.name = 'website_id'`).Scan(&tables).Error
	if err != nil {
		return 0, fmt.Errorf("failed to list tables with a website_id: %w", err)
	}

	var total int64
	for _, id := range ids {
		for _, table := range tables {
			deleted, err := purgeRows(dbManager, table, id)
			total += deleted
			if err != nil {
				return total, fmt.Errorf("failed to delete %s of website %d: %w", table, id, err)
			}
		}
	}
	return total, nil
}

// deletedWebsiteIDs returns the IDs of the deleted websites. SQLite never
// gives out a website ID twice (AUTOINCREMENT), so an ID up to the highest
// one given out that has no website is the ID of a deleted website.
func deletedWebsiteIDs(db *gorm.DB) ([]uint, error) {
	var highest uint
	err := db.Raw(`SELECT COALESCE(MAX(seq), 0) FROM sqlite_sequence WHERE name = 'websites'`).Scan(&highest).Error
	if err != nil {
		return nil, fmt.Errorf("failed to read the highest website ID: %w", err)
	}

	var existing []uint
	if err := db.Model(&Website{}).Pluck("id", &existing).Error; err != nil {
		return nil, fmt.Errorf("failed to list websites: %w", err)
	}
	alive := make(map[uint]bool, len(existing))
	for _, id := range existing {
		alive[id] = true
	}

	var deleted []uint
	for id := uint(1); id <= highest; id++ {
		if !alive[id] {
			deleted = append(deleted, id)
		}
	}
	return deleted, nil
}

// purgeRows deletes one website's rows from one table.
func purgeRows(dbManager cartridge.DBManager, table string, websiteID uint) (int64, error) {
	query := fmt.Sprintf("DELETE FROM `%s` WHERE rowid IN (SELECT rowid FROM `%s` WHERE website_id = ? LIMIT ?)", table, table)
	return batch.Delete(dbManager, query, websiteID)
}
