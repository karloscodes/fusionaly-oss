package annotations

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	// Auto-migrate the Annotation model
	if err := db.AutoMigrate(&Annotation{}); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func TestCreateAnnotation(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		name        string
		annotation  *Annotation
		wantErr     bool
		errContains string
	}{
		{
			name: "valid annotation",
			annotation: &Annotation{
				WebsiteID:      1,
				Title:          "Test Annotation",
				Description:    "Test description",
				AnnotationType: AnnotationGeneral,
				AnnotationDate: time.Now(),
				Color:          "#ff0000",
			},
			wantErr: false,
		},
		{
			name: "valid annotation with defaults",
			annotation: &Annotation{
				WebsiteID:      1,
				Title:          "Minimal Annotation",
				AnnotationDate: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing title",
			annotation: &Annotation{
				WebsiteID:      1,
				AnnotationDate: time.Now(),
			},
			wantErr:     true,
			errContains: "title is required",
		},
		{
			name: "missing website ID",
			annotation: &Annotation{
				Title:          "Test",
				AnnotationDate: time.Now(),
			},
			wantErr:     true,
			errContains: "website ID is required",
		},
		{
			name: "missing date",
			annotation: &Annotation{
				WebsiteID: 1,
				Title:     "Test",
			},
			wantErr:     true,
			errContains: "date is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateAnnotation(db, tt.annotation)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.annotation.ID == 0 {
					t.Error("expected annotation ID to be set")
				}
				if tt.annotation.CreatedAt.IsZero() {
					t.Error("expected CreatedAt to be set")
				}
			}
		})
	}
}

func TestCreateAnnotation_SetsDefaults(t *testing.T) {
	db := setupTestDB(t)

	annotation := &Annotation{
		WebsiteID:      1,
		Title:          "Test",
		AnnotationDate: time.Now(),
	}

	err := CreateAnnotation(db, annotation)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if annotation.AnnotationType != AnnotationGeneral {
		t.Errorf("expected default type %q, got %q", AnnotationGeneral, annotation.AnnotationType)
	}

	if annotation.Color != "#6366f1" {
		t.Errorf("expected default color #6366f1, got %q", annotation.Color)
	}
}

func TestGetAnnotationByID(t *testing.T) {
	db := setupTestDB(t)

	// Create test annotation
	annotation := &Annotation{
		WebsiteID:      1,
		Title:          "Test Annotation",
		AnnotationDate: time.Now(),
	}
	if err := CreateAnnotation(db, annotation); err != nil {
		t.Fatalf("failed to create annotation: %v", err)
	}

	tests := []struct {
		name      string
		id        uint
		websiteID uint
		wantErr   bool
	}{
		{
			name:      "existing annotation",
			id:        annotation.ID,
			websiteID: 1,
			wantErr:   false,
		},
		{
			name:      "non-existent ID",
			id:        99999,
			websiteID: 1,
			wantErr:   true,
		},
		{
			name:      "wrong website ID",
			id:        annotation.ID,
			websiteID: 999,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetAnnotationByID(db, tt.id, tt.websiteID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Title != annotation.Title {
					t.Errorf("expected title %q, got %q", annotation.Title, result.Title)
				}
			}
		})
	}
}

func TestGetAnnotationsForWebsite(t *testing.T) {
	db := setupTestDB(t)

	// Create test annotations for website 1
	for i := 0; i < 3; i++ {
		annotation := &Annotation{
			WebsiteID:      1,
			Title:          "Website 1 Annotation",
			AnnotationDate: time.Now().Add(time.Duration(i) * time.Hour),
		}
		if err := CreateAnnotation(db, annotation); err != nil {
			t.Fatalf("failed to create annotation: %v", err)
		}
	}

	// Create annotation for website 2
	annotation2 := &Annotation{
		WebsiteID:      2,
		Title:          "Website 2 Annotation",
		AnnotationDate: time.Now(),
	}
	if err := CreateAnnotation(db, annotation2); err != nil {
		t.Fatalf("failed to create annotation: %v", err)
	}

	// Get annotations for website 1
	results, err := GetAnnotationsForWebsite(db, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 annotations, got %d", len(results))
	}

	// Get annotations for website 2
	results2, err := GetAnnotationsForWebsite(db, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results2) != 1 {
		t.Errorf("expected 1 annotation, got %d", len(results2))
	}
}

func TestGetAnnotationsForTimeframe(t *testing.T) {
	db := setupTestDB(t)

	// Use UTC for consistent timezone handling
	now := time.Now().UTC()

	// Create annotations at different times
	dates := []time.Time{
		now.Add(-48 * time.Hour), // 2 days ago - outside range
		now.Add(-24 * time.Hour), // 1 day ago - inside range
		now.Add(-1 * time.Hour),  // 1 hour ago - inside range
		now.Add(48 * time.Hour),  // 2 days from now - outside range
	}

	for i, date := range dates {
		annotation := &Annotation{
			WebsiteID:      1,
			Title:          "Annotation " + string(rune('A'+i)),
			AnnotationDate: date,
		}
		if err := CreateAnnotation(db, annotation); err != nil {
			t.Fatalf("failed to create annotation: %v", err)
		}
	}

	// Query for last 36 hours (should include annotations B and C)
	from := now.Add(-36 * time.Hour)
	to := now.Add(time.Hour)

	results, err := GetAnnotationsForTimeframe(db, 1, from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 annotations in timeframe, got %d", len(results))
		for _, r := range results {
			t.Logf("  - %s: %v", r.Title, r.AnnotationDate)
		}
	}
}

func TestUpdateAnnotation(t *testing.T) {
	db := setupTestDB(t)

	// Create test annotation
	annotation := &Annotation{
		WebsiteID:      1,
		Title:          "Original Title",
		Description:    "Original description",
		AnnotationDate: time.Now(),
		Color:          "#ff0000",
	}
	if err := CreateAnnotation(db, annotation); err != nil {
		t.Fatalf("failed to create annotation: %v", err)
	}

	// Update annotation
	annotation.Title = "Updated Title"
	annotation.Description = "Updated description"
	annotation.Color = "#00ff00"

	if err := UpdateAnnotation(db, annotation); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify update
	result, err := GetAnnotationByID(db, annotation.ID, 1)
	if err != nil {
		t.Fatalf("failed to get annotation: %v", err)
	}

	if result.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", result.Title)
	}
	if result.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", result.Description)
	}
	if result.Color != "#00ff00" {
		t.Errorf("expected color '#00ff00', got %q", result.Color)
	}
}

func TestUpdateAnnotation_Validation(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		name        string
		annotation  *Annotation
		errContains string
	}{
		{
			name:        "missing ID",
			annotation:  &Annotation{Title: "Test"},
			errContains: "ID is required",
		},
		{
			name:        "missing title",
			annotation:  &Annotation{ID: 1},
			errContains: "title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateAnnotation(db, tt.annotation)
			if err == nil {
				t.Error("expected error, got nil")
			} else if !containsString(err.Error(), tt.errContains) {
				t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

func TestDeleteAnnotation(t *testing.T) {
	db := setupTestDB(t)

	// Create test annotation
	annotation := &Annotation{
		WebsiteID:      1,
		Title:          "To Delete",
		AnnotationDate: time.Now(),
	}
	if err := CreateAnnotation(db, annotation); err != nil {
		t.Fatalf("failed to create annotation: %v", err)
	}

	// Delete annotation
	err := DeleteAnnotation(db, annotation.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deletion
	_, err = GetAnnotationByID(db, annotation.ID, 1)
	if err == nil {
		t.Error("expected error when getting deleted annotation")
	}
}

func TestDeleteAnnotation_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := DeleteAnnotation(db, 99999, 1)
	if err == nil {
		t.Error("expected error for non-existent annotation")
	}
}

func TestDeleteAnnotation_WrongWebsite(t *testing.T) {
	db := setupTestDB(t)

	// Create annotation for website 1
	annotation := &Annotation{
		WebsiteID:      1,
		Title:          "Test",
		AnnotationDate: time.Now(),
	}
	if err := CreateAnnotation(db, annotation); err != nil {
		t.Fatalf("failed to create annotation: %v", err)
	}

	// Try to delete with wrong website ID
	err := DeleteAnnotation(db, annotation.ID, 999)
	if err == nil {
		t.Error("expected error when deleting with wrong website ID")
	}

	// Verify annotation still exists
	_, err = GetAnnotationByID(db, annotation.ID, 1)
	if err != nil {
		t.Error("annotation should still exist")
	}
}

func TestIsValidAnnotationType(t *testing.T) {
	tests := []struct {
		name     string
		typ      AnnotationType
		expected bool
	}{
		{"deployment", AnnotationDeployment, true},
		{"campaign", AnnotationCampaign, true},
		{"incident", AnnotationIncident, true},
		{"general", AnnotationGeneral, true},
		{"invalid", AnnotationType("invalid"), false},
		{"empty", AnnotationType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAnnotationType(tt.typ)
			if result != tt.expected {
				t.Errorf("IsValidAnnotationType(%q) = %v, want %v", tt.typ, result, tt.expected)
			}
		})
	}
}

func TestGetAnnotationTypeColor(t *testing.T) {
	tests := []struct {
		name     string
		typ      AnnotationType
		expected string
	}{
		{"deployment", AnnotationDeployment, "#22c55e"},
		{"campaign", AnnotationCampaign, "#3b82f6"},
		{"incident", AnnotationIncident, "#ef4444"},
		{"general", AnnotationGeneral, "#6366f1"},
		{"unknown", AnnotationType("unknown"), "#6366f1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAnnotationTypeColor(tt.typ)
			if result != tt.expected {
				t.Errorf("GetAnnotationTypeColor(%q) = %q, want %q", tt.typ, result, tt.expected)
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
