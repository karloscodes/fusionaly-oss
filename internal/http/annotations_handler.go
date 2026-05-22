package http

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"log/slog"

	"fusionaly/internal/annotations"
	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/flash"
)

// annotationFormData holds parsed form data for annotations
type annotationFormData struct {
	Title          string
	Description    string
	AnnotationType string
	AnnotationDate string
	Color          string
}

// parseAnnotationForm extracts annotation data from form values or JSON body.
// Bind is content-type aware (form-encoded or Inertia.js JSON).
func parseAnnotationForm(ctx *cartridge.Context) annotationFormData {
	var in struct {
		Title          string `json:"title" form:"title"`
		Description    string `json:"description" form:"description"`
		AnnotationType string `json:"annotation_type" form:"annotation_type"`
		AnnotationDate string `json:"annotation_date" form:"annotation_date"`
		Color          string `json:"color" form:"color"`
	}
	_ = ctx.Bind(&in)
	return annotationFormData{
		Title:          in.Title,
		Description:    in.Description,
		AnnotationType: in.AnnotationType,
		AnnotationDate: in.AnnotationDate,
		Color:          in.Color,
	}
}

// annotationDateFormats defines accepted date formats for annotation dates
var annotationDateFormats = []string{
	"2006-01-02T15:04", // HTML datetime-local format
	"2006-01-02",       // Simple date format
	time.RFC3339,       // ISO format
}

// parseAnnotationDate parses a date string using multiple formats
func parseAnnotationDate(dateStr string) (time.Time, bool) {
	for _, format := range annotationDateFormats {
		if parsed, err := time.Parse(format, dateStr); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// dashboardPath returns the dashboard URL path for a website
func dashboardPath(websiteID int) string {
	return "/admin/websites/" + strconv.Itoa(websiteID) + "/dashboard"
}

// AnnotationsListAction returns annotations for a website (JSON API)
func AnnotationsListAction(ctx *cartridge.Context) error {
	websiteID, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid website ID",
		})
	}

	db := ctx.DB()

	// Parse optional from/to query params for timeframe filtering
	fromStr := ctx.Query("from")
	toStr := ctx.Query("to")

	var annotationsList []annotations.Annotation

	if fromStr != "" && toStr != "" {
		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			ctx.Logger.Warn("Invalid from date format", slog.String("from", fromStr), slog.Any("error", err))
			from = time.Now().AddDate(0, -1, 0) // Default to 1 month ago
		}

		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			ctx.Logger.Warn("Invalid to date format", slog.String("to", toStr), slog.Any("error", err))
			to = time.Now()
		}

		annotationsList, err = annotations.GetAnnotationsForTimeframe(db, uint(websiteID), from, to)
		if err != nil {
			ctx.Logger.Error("Failed to get annotations for timeframe", slog.Any("error", err), slog.Int("websiteID", websiteID))
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch annotations",
			})
		}
	} else {
		annotationsList, err = annotations.GetAnnotationsForWebsite(db, uint(websiteID))
		if err != nil {
			ctx.Logger.Error("Failed to get annotations", slog.Any("error", err), slog.Int("websiteID", websiteID))
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch annotations",
			})
		}
	}

	return ctx.JSON(fiber.Map{
		"annotations": annotationsList,
	})
}

// AnnotationCreateAction creates a new annotation (form submission)
func AnnotationCreateAction(ctx *cartridge.Context) error {
	websiteID, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.FlashError("Invalid website ID").Redirect("/admin", fiber.StatusFound)
	}

	form := parseAnnotationForm(ctx)
	redirectPath := dashboardPath(websiteID)

	ctx.Logger.Info("Creating annotation",
		slog.Int("websiteID", websiteID),
		slog.String("title", form.Title),
		slog.String("type", form.AnnotationType),
		slog.String("date", form.AnnotationDate),
	)

	// Validate required fields
	if form.Title == "" {
		return ctx.FlashError("Title is required").Redirect(redirectPath, fiber.StatusFound)
	}

	if form.AnnotationDate == "" {
		return ctx.FlashError("Date is required").Redirect(redirectPath, fiber.StatusFound)
	}

	annotationDate, ok := parseAnnotationDate(form.AnnotationDate)
	if !ok {
		ctx.Logger.Error("Failed to parse annotation date", slog.String("date", form.AnnotationDate))
		return ctx.FlashError("Invalid date format").Redirect(redirectPath, fiber.StatusFound)
	}

	db := ctx.DB()

	annotation := &annotations.Annotation{
		WebsiteID:      uint(websiteID),
		Title:          form.Title,
		Description:    form.Description,
		AnnotationType: annotations.AnnotationType(form.AnnotationType),
		AnnotationDate: annotationDate,
		Color:          form.Color,
	}

	// Use default color based on type if not provided
	if annotation.Color == "" {
		annotation.Color = annotations.GetAnnotationTypeColor(annotation.AnnotationType)
	}

	if err := annotations.CreateAnnotation(db, annotation); err != nil {
		ctx.Logger.Error("Failed to create annotation", slog.Any("error", err))
		return ctx.FlashError("Failed to create annotation").Redirect(redirectPath, fiber.StatusFound)
	}

	ctx.Logger.Info("Annotation created successfully",
		slog.Uint64("id", uint64(annotation.ID)),
		slog.Int("websiteID", websiteID),
	)

	return ctx.FlashSuccess("Annotation created successfully").Redirect(redirectPath, fiber.StatusFound)
}

// AnnotationUpdateAction updates an existing annotation (form submission)
func AnnotationUpdateAction(ctx *cartridge.Context) error {
	websiteID, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.FlashError("Invalid website ID").Redirect("/admin", fiber.StatusFound)
	}

	redirectPath := dashboardPath(websiteID)

	annotationID, err := ctx.ParamsInt("annotationId")
	if err != nil {
		ctx.Logger.Error("Invalid annotation ID", slog.Any("error", err))
		return ctx.FlashError("Invalid annotation ID").Redirect(redirectPath, fiber.StatusFound)
	}

	db := ctx.DB()

	existing, err := annotations.GetAnnotationByID(db, uint(annotationID), uint(websiteID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			flash.SetFlash(ctx.Ctx, "error", "Annotation not found")
		} else {
			ctx.Logger.Error("Failed to get annotation", slog.Any("error", err))
			flash.SetFlash(ctx.Ctx, "error", "Failed to update annotation")
		}
		return ctx.Redirect(redirectPath, fiber.StatusFound)
	}

	form := parseAnnotationForm(ctx)

	if form.Title == "" {
		return ctx.FlashError("Title is required").Redirect(redirectPath, fiber.StatusFound)
	}

	// Update fields
	existing.Title = form.Title
	existing.Description = form.Description
	if form.AnnotationType != "" {
		existing.AnnotationType = annotations.AnnotationType(form.AnnotationType)
	}
	if form.Color != "" {
		existing.Color = form.Color
	}
	if form.AnnotationDate != "" {
		if parsed, ok := parseAnnotationDate(form.AnnotationDate); ok {
			existing.AnnotationDate = parsed
		}
	}

	if err := annotations.UpdateAnnotation(db, existing); err != nil {
		ctx.Logger.Error("Failed to update annotation", slog.Any("error", err))
		return ctx.FlashError("Failed to update annotation").Redirect(redirectPath, fiber.StatusFound)
	}

	ctx.Logger.Info("Annotation updated successfully",
		slog.Uint64("id", uint64(existing.ID)),
		slog.Int("websiteID", websiteID),
	)

	return ctx.FlashSuccess("Annotation updated successfully").Redirect(redirectPath, fiber.StatusFound)
}

// AnnotationDeleteAction deletes an annotation (form submission)
func AnnotationDeleteAction(ctx *cartridge.Context) error {
	websiteID, err := ctx.ParamsInt("id")
	if err != nil {
		ctx.Logger.Error("Invalid website ID", slog.Any("error", err))
		return ctx.FlashError("Invalid website ID").Redirect("/admin", fiber.StatusFound)
	}

	redirectPath := dashboardPath(websiteID)

	annotationID, err := ctx.ParamsInt("annotationId")
	if err != nil {
		ctx.Logger.Error("Invalid annotation ID", slog.Any("error", err))
		return ctx.FlashError("Invalid annotation ID").Redirect(redirectPath, fiber.StatusFound)
	}

	db := ctx.DB()

	if err := annotations.DeleteAnnotation(db, uint(annotationID), uint(websiteID)); err != nil {
		if err == gorm.ErrRecordNotFound {
			flash.SetFlash(ctx.Ctx, "error", "Annotation not found")
		} else {
			ctx.Logger.Error("Failed to delete annotation", slog.Any("error", err))
			flash.SetFlash(ctx.Ctx, "error", "Failed to delete annotation")
		}
		return ctx.Redirect(redirectPath, fiber.StatusFound)
	}

	ctx.Logger.Info("Annotation deleted successfully",
		slog.Int("annotationID", annotationID),
		slog.Int("websiteID", websiteID),
	)

	return ctx.FlashSuccess("Annotation deleted successfully").Redirect(redirectPath, fiber.StatusFound)
}
