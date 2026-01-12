package annotations

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AnnotationType represents the type of annotation
type AnnotationType string

const (
	AnnotationDeployment AnnotationType = "deployment"
	AnnotationCampaign   AnnotationType = "campaign"
	AnnotationIncident   AnnotationType = "incident"
	AnnotationGeneral    AnnotationType = "general"
)

// Annotation represents a marker on the dashboard timeline
type Annotation struct {
	ID             uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	WebsiteID      uint           `gorm:"not null;index:idx_annotations_website_date" json:"website_id"`
	Title          string         `gorm:"not null;size:255" json:"title"`
	Description    string         `gorm:"size:1000" json:"description"`
	AnnotationType AnnotationType `gorm:"size:50;default:'general'" json:"annotation_type"`
	AnnotationDate time.Time      `gorm:"not null;index:idx_annotations_website_date" json:"annotation_date"`
	Color          string         `gorm:"size:20;default:'#6366f1'" json:"color"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// TableName specifies the table name for GORM
func (Annotation) TableName() string {
	return "annotations"
}

// ValidAnnotationTypes returns all valid annotation types
func ValidAnnotationTypes() []AnnotationType {
	return []AnnotationType{
		AnnotationDeployment,
		AnnotationCampaign,
		AnnotationIncident,
		AnnotationGeneral,
	}
}

// IsValidAnnotationType checks if the given type is valid
func IsValidAnnotationType(t AnnotationType) bool {
	for _, valid := range ValidAnnotationTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// CreateAnnotation creates a new annotation in the database
func CreateAnnotation(db *gorm.DB, annotation *Annotation) error {
	if annotation.Title == "" {
		return fmt.Errorf("annotation title is required")
	}
	if annotation.WebsiteID == 0 {
		return fmt.Errorf("website ID is required")
	}
	if annotation.AnnotationDate.IsZero() {
		return fmt.Errorf("annotation date is required")
	}

	// Set defaults
	if annotation.AnnotationType == "" {
		annotation.AnnotationType = AnnotationGeneral
	}
	if annotation.Color == "" {
		annotation.Color = "#6366f1" // indigo-500
	}

	now := time.Now().UTC()
	annotation.CreatedAt = now
	annotation.UpdatedAt = now

	return db.Create(annotation).Error
}

// GetAnnotationByID retrieves an annotation by ID and website ID
func GetAnnotationByID(db *gorm.DB, id uint, websiteID uint) (*Annotation, error) {
	var annotation Annotation
	err := db.Where("id = ? AND website_id = ?", id, websiteID).First(&annotation).Error
	if err != nil {
		return nil, err
	}
	return &annotation, nil
}

// GetAnnotationsForWebsite retrieves all annotations for a website
func GetAnnotationsForWebsite(db *gorm.DB, websiteID uint) ([]Annotation, error) {
	var annotations []Annotation
	err := db.Where("website_id = ?", websiteID).
		Order("annotation_date DESC").
		Find(&annotations).Error
	if err != nil {
		return nil, err
	}
	return annotations, nil
}

// GetAnnotationsForTimeframe retrieves annotations within a specific time range
func GetAnnotationsForTimeframe(db *gorm.DB, websiteID uint, from, to time.Time) ([]Annotation, error) {
	var annotations []Annotation
	err := db.Where("website_id = ? AND annotation_date >= ? AND annotation_date <= ?",
		websiteID, from.UTC(), to.UTC()).
		Order("annotation_date ASC").
		Find(&annotations).Error
	if err != nil {
		return nil, err
	}
	return annotations, nil
}

// UpdateAnnotation updates an existing annotation
func UpdateAnnotation(db *gorm.DB, annotation *Annotation) error {
	if annotation.ID == 0 {
		return fmt.Errorf("annotation ID is required")
	}
	if annotation.Title == "" {
		return fmt.Errorf("annotation title is required")
	}

	annotation.UpdatedAt = time.Now().UTC()

	// Only update specific fields to prevent overwriting website_id
	return db.Model(annotation).
		Select("title", "description", "annotation_type", "annotation_date", "color", "updated_at").
		Updates(annotation).Error
}

// DeleteAnnotation deletes an annotation by ID and website ID
func DeleteAnnotation(db *gorm.DB, id uint, websiteID uint) error {
	result := db.Where("id = ? AND website_id = ?", id, websiteID).Delete(&Annotation{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetAnnotationTypeColor returns a default color for each annotation type
func GetAnnotationTypeColor(t AnnotationType) string {
	switch t {
	case AnnotationDeployment:
		return "#22c55e" // green-500
	case AnnotationCampaign:
		return "#3b82f6" // blue-500
	case AnnotationIncident:
		return "#ef4444" // red-500
	default:
		return "#6366f1" // indigo-500
	}
}
