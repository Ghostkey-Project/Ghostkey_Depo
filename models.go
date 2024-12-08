package main

import (
	"time"

	"gorm.io/gorm"
)

// StoredFile represents a file stored in the system
type StoredFile struct {
	gorm.Model
	FileName           string    `gorm:"not null"`
	FilePath           string    `gorm:"not null"`
	EspID              string    `gorm:"not null"`
	DeliveryKey        string    `gorm:"not null"`
	EncryptionPassword string    `gorm:"not null"`
	FileSize           int64     `gorm:"not null"`
	UploadTime         time.Time `gorm:"not null"`
	Analyzed           bool      `gorm:"default:false"`
}

// AnalysisResult stores the results of file analysis
type AnalysisResult struct {
	gorm.Model
	FileID     uint       `gorm:"not null"`
	FileName   string     `gorm:"not null"`
	StoredFile StoredFile `gorm:"foreignKey:FileID"`
	Parameters string     `gorm:"type:json"`      // JSON string of parameters used
	BasicInfo  string     `gorm:"type:json"`      // Basic file information
	EspInfo    string     `gorm:"type:json"`      // ESP-related information
	Metadata   string     `gorm:"type:json"`      // File metadata from exiftool
	Analysis   string     `gorm:"type:json"`      // Content analysis results
	Status     string     `gorm:"not null"`       // "pending", "completed", "failed"
	StartTime  time.Time  `gorm:"not null"`
	EndTime    *time.Time
	Error      *string
}
