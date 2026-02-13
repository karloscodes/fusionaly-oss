package settings

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/karloscodes/cartridge/cache"
	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"
)

// Setting represents a configuration item in the database
type Setting struct {
	ID        uint      `gorm:"primaryKey"`
	Key       string    `gorm:"uniqueIndex;not null"`
	Value     string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null;autoCreateTime:milli"`
	UpdatedAt time.Time `gorm:"not null;autoUpdateTime:milli"`
}

var excludedIPsCache *cache.Cache[string, []string]

// SetupDefaultSettings initializes default settings in the database
func SetupDefaultSettings(dbConn *gorm.DB) error {
	settings := []Setting{
		{Key: "excluded_ips", Value: ""},
		{Key: "subdomain_tracking", Value: "{}"},
		{Key: "website_goals", Value: "{\"goals\":{}}"},
	}
	err := sqlite.PerformWrite(slog.Default(), dbConn, func(tx *gorm.DB) error {
		for _, setting := range settings {
			// Use raw SQL for upsert
			err := tx.Exec(`
                INSERT INTO settings (key, value, created_at, updated_at)
                VALUES (?, ?, ?, ?)
                ON CONFLICT(key) DO NOTHING
            `, setting.Key, setting.Value, time.Now().UTC(), time.Now().UTC()).Error
			if err != nil {
				slog.Default().Error("Failed to upsert setting", slog.String("key", setting.Key), slog.Any("error", err))
				return fmt.Errorf("failed to upsert setting %s: %w", setting.Key, err)
			}
		}
		return nil
	})

	// Initialize the cache
	loadCache(dbConn, slog.Default())

	return err
}

func IsIPExcluded(ip string) (bool, error) {
	// If the cache isn't initialized yet, return false
	if excludedIPsCache == nil {
		return false, nil
	}

	// Fetch excluded IPs from the cache
	excludedIPs, err := excludedIPsCache.Get("excluded_ips")
	if err != nil {
		return false, fmt.Errorf("failed to check excluded IPs: %w", err)
	}

	// Check if the IP is in the excluded list
	for _, excludedIP := range excludedIPs {
		if excludedIP == ip {
			return true, nil
		}
	}
	return false, nil
}

// GetSetting retrieves a setting value from the database
func GetSetting(dbConn *gorm.DB, key string) (string, error) {
	var setting Setting
	result := dbConn.Where("key = ?", key).First(&setting)

	if result.Error != nil {
		return "", result.Error
	}

	return setting.Value, nil
}

// UpdateSetting updates a setting in the database using a transaction
func UpdateSetting(dbConn *gorm.DB, key string, value string) error {
	// Start a transaction
	tx := dbConn.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Ensure we either commit or rollback
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update the setting within the transaction
	result := tx.Model(&Setting{}).Where("key = ?", key).Update("value", value)
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update setting: %w", result.Error)
	}

	// If no rows were affected, the setting might not exist - try to create it
	if result.RowsAffected == 0 {
		setting := Setting{
			Key:   key,
			Value: value,
		}
		if err := tx.Create(&setting).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create setting: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Clear and reload the cache after successful update
	excludedIPsCache.Clear()
	loadCache(dbConn, slog.Default())

	return nil
}

// CreateOrUpdateSetting creates a new setting or updates an existing one
func CreateOrUpdateSetting(dbConn *gorm.DB, key string, value string) error {
	// Check if the setting exists
	var count int64
	if err := dbConn.Model(&Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check if setting exists: %w", err)
	}

	// If it exists, update it; otherwise, create it
	if count > 0 {
		return UpdateSetting(dbConn, key, value)
	} else {
		setting := Setting{
			Key:   key,
			Value: value,
		}
		if err := dbConn.Create(&setting).Error; err != nil {
			return fmt.Errorf("failed to create setting: %w", err)
		}
		return nil
	}
}

// Note: GetOpenAiApiKey is available in Fusionaly Pro

// Setup initializes the models package with the database and logger.
func loadCache(dbConn *gorm.DB, logger *slog.Logger) {
	// Initialize the excluded IPs cache
	fetchFunc := func(key string) ([]string, error) {
		var value string
		err := dbConn.WithContext(context.Background()).Raw("SELECT value FROM settings WHERE key = ? LIMIT 1", key).Scan(&value).Error
		if err != nil {
			return nil, err
		}
		// Parse the excluded IPs (assuming it's a comma-separated list)
		excludedIPs := strings.Split(value, ",")
		for i, ip := range excludedIPs {
			excludedIPs[i] = strings.TrimSpace(ip)
		}
		return excludedIPs, nil
	}
	excludedIPsCache = cache.NewCache[string, []string](logger, 5*time.Minute, fetchFunc)
}

// GetSubdomainTrackingSettings retrieves subdomain tracking settings from the database
func GetSubdomainTrackingSettings(dbConn *gorm.DB) (map[string]bool, error) {
	settingsJSON, err := GetSetting(dbConn, "subdomain_tracking")
	if err != nil {
		return map[string]bool{}, nil // Return empty map if not found
	}

	var settings map[string]bool
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return map[string]bool{}, nil // Return empty map if invalid JSON
	}

	return settings, nil
}

// IsSubdomainTrackingEnabled checks if subdomain tracking is enabled for a specific domain
func IsSubdomainTrackingEnabled(dbConn *gorm.DB, domain string) bool {
	settings, err := GetSubdomainTrackingSettings(dbConn)
	if err != nil {
		return false
	}

	return settings[domain]
}

// UpdateSubdomainTrackingSettings updates subdomain tracking settings for a domain
func UpdateSubdomainTrackingSettings(dbConn *gorm.DB, domain string, enabled bool) error {
	settings, err := GetSubdomainTrackingSettings(dbConn)
	if err != nil {
		settings = make(map[string]bool)
	}

	settings[domain] = enabled

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal subdomain tracking settings: %w", err)
	}

	return UpdateSetting(dbConn, "subdomain_tracking", string(settingsJSON))
}

// WebsiteGoals represents the structure for storing conversion goals per website
type WebsiteGoals struct {
	Goals map[string][]string `json:"goals"` // Map of website ID (as string) to goals array
}

// GetWebsiteGoals retrieves conversion goals for a specific website
func GetWebsiteGoals(db *gorm.DB, websiteID uint) ([]string, error) {
	if websiteID == 0 {
		return []string{}, nil
	}

	websiteGoalsJSON, err := GetSetting(db, "website_goals")
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			const emptyGoalsJSON = `{"goals":{}}`
			if err := CreateOrUpdateSetting(db, "website_goals", emptyGoalsJSON); err != nil {
				return []string{}, nil
			}
			return []string{}, nil
		}
		return []string{}, nil
	}

	if websiteGoalsJSON == "" {
		return []string{}, nil
	}

	var websiteGoals WebsiteGoals
	if err := json.Unmarshal([]byte(websiteGoalsJSON), &websiteGoals); err != nil {
		return []string{}, nil
	}

	if websiteGoals.Goals == nil {
		websiteGoals.Goals = make(map[string][]string)
		updatedJSON, _ := json.Marshal(websiteGoals)
		_ = UpdateSetting(db, "website_goals", string(updatedJSON))
	}

	websiteIDStr := strconv.FormatUint(uint64(websiteID), 10)
	if goals, ok := websiteGoals.Goals[websiteIDStr]; ok {
		return goals, nil
	}

	return []string{}, nil
}

// SaveWebsiteGoals saves conversion goals for a specific website
func SaveWebsiteGoals(db *gorm.DB, websiteID uint, goals []string) error {
	websiteGoalsJSON, err := GetSetting(db, "website_goals")
	var websiteGoals WebsiteGoals

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			websiteGoals = WebsiteGoals{Goals: make(map[string][]string)}
		} else {
			return err
		}
	} else if websiteGoalsJSON != "" {
		if err := json.Unmarshal([]byte(websiteGoalsJSON), &websiteGoals); err != nil {
			websiteGoals = WebsiteGoals{Goals: make(map[string][]string)}
		}
	}

	if websiteGoals.Goals == nil {
		websiteGoals.Goals = make(map[string][]string)
	}

	cleanedGoals := make([]string, 0, len(goals))
	goalMap := make(map[string]bool)
	for _, goal := range goals {
		cleanedGoal := strings.TrimSpace(goal)
		if cleanedGoal != "" && !goalMap[cleanedGoal] {
			goalMap[cleanedGoal] = true
			cleanedGoals = append(cleanedGoals, cleanedGoal)
		}
	}

	websiteIDStr := strconv.FormatUint(uint64(websiteID), 10)
	websiteGoals.Goals[websiteIDStr] = cleanedGoals

	updatedJSON, err := json.Marshal(websiteGoals)
	if err != nil {
		return fmt.Errorf("failed to marshal website_goals: %w", err)
	}

	if err := CreateOrUpdateSetting(db, "website_goals", string(updatedJSON)); err != nil {
		return fmt.Errorf("failed to save website_goals setting: %w", err)
	}

	return nil
}

// SettingResponse represents a setting key-value pair for API responses
type SettingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// GeoLite settings keys
const (
	KeyGeoLiteAccountID  = "geolite_account_id"
	KeyGeoLiteLicenseKey = "geolite_license_key"
)

// Agent API settings keys
const (
	KeyAgentAPIKey = "agent_api_key"
)

// GetGeoLiteCredentials retrieves GeoLite account ID and license key
func GetGeoLiteCredentials(db *gorm.DB) (accountID string, licenseKey string, err error) {
	accountID, _ = GetSetting(db, KeyGeoLiteAccountID)
	licenseKey, _ = GetSetting(db, KeyGeoLiteLicenseKey)
	return accountID, licenseKey, nil
}

// SaveGeoLiteCredentials saves GeoLite account ID and license key
func SaveGeoLiteCredentials(db *gorm.DB, accountID string, licenseKey string) error {
	if err := CreateOrUpdateSetting(db, KeyGeoLiteAccountID, strings.TrimSpace(accountID)); err != nil {
		return fmt.Errorf("failed to save GeoLite account ID: %w", err)
	}
	if err := CreateOrUpdateSetting(db, KeyGeoLiteLicenseKey, strings.TrimSpace(licenseKey)); err != nil {
		return fmt.Errorf("failed to save GeoLite license key: %w", err)
	}
	return nil
}

// GetAgentAPIKey retrieves the agent API key
func GetAgentAPIKey(db *gorm.DB) (string, error) {
	return GetSetting(db, KeyAgentAPIKey)
}

// GetOrCreateAgentAPIKey returns the existing API key or generates a new one
func GetOrCreateAgentAPIKey(db *gorm.DB) (string, error) {
	key, err := GetAgentAPIKey(db)
	if err == nil && key != "" {
		return key, nil
	}
	return GenerateAgentAPIKey(db)
}

// GenerateAgentAPIKey creates a new random API key and stores it
func GenerateAgentAPIKey(db *gorm.DB) (string, error) {
	key := generateRandomToken(32)
	if err := CreateOrUpdateSetting(db, KeyAgentAPIKey, key); err != nil {
		return "", err
	}
	return key, nil
}

// RegenerateAgentAPIKey creates a new API key, replacing the old one
func RegenerateAgentAPIKey(db *gorm.DB) (string, error) {
	return GenerateAgentAPIKey(db)
}

// generateRandomToken creates a cryptographically secure random token
func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[randInt(len(charset))]
	}
	return string(b)
}

// randInt returns a cryptographically secure random int in [0, max)
func randInt(max int) int {
	var buf [1]byte
	_, _ = rand.Read(buf[:])
	return int(buf[0]) % max
}

// GetAllSettingsForDisplay retrieves all general (non-website-specific) settings
// with sensitive values masked for display
func GetAllSettingsForDisplay(db *gorm.DB) ([]SettingResponse, error) {
	var allSettings []Setting
	if err := db.Find(&allSettings).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch settings: %w", err)
	}

	var result []SettingResponse
	for _, setting := range allSettings {
		// Include only general settings (not per-website, except website_goals)
		if !strings.HasPrefix(setting.Key, "website_") || setting.Key == "website_goals" {
			value := setting.Value
			// Mask sensitive keys for display
			if setting.Key == "license_key" && value != "" {
				value = strings.Repeat("*", len(value))
			}
			result = append(result, SettingResponse{
				Key:   setting.Key,
				Value: value,
			})
		}
	}
	return result, nil
}
