package models

type Profile struct {
	ID       uint `gorm:"primaryKey;autoIncrement"`
	UserID   uint `gorm:"not null;uniqueIndex"`
	Metadata JSON
}
