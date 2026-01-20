package websites

import (
	"crypto/rand"
	"encoding/base64"

	"gorm.io/gorm"
)

// generateToken creates a URL-safe random token of the specified length
func generateToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// EnableSharing generates a share token for the website, enabling public access
func EnableSharing(db *gorm.DB, websiteID uint) (string, error) {
	token := generateToken(12)
	err := db.Model(&Website{}).
		Where("id = ?", websiteID).
		Update("share_token", token).Error
	return token, err
}

// DisableSharing removes the share token, invalidating the public URL
func DisableSharing(db *gorm.DB, websiteID uint) error {
	return db.Model(&Website{}).
		Where("id = ?", websiteID).
		Update("share_token", nil).Error
}

// GetWebsiteByShareToken finds a website by its public share token
func GetWebsiteByShareToken(db *gorm.DB, token string) (*Website, error) {
	var website Website
	err := db.Where("share_token = ?", token).First(&website).Error
	if err != nil {
		return nil, err
	}
	return &website, nil
}
