package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// AnalysisParams holds the configuration for file analysis
type AnalysisParams struct {
	MaxFileSize        string              `json:"max_file_size"`
	AllowedExtensions  string              `json:"allowed_extensions"`
	ScanTimeout        string              `json:"scan_timeout"`
	ContentCheck       string              `json:"content_check"`
	VirusScan          string              `json:"virus_scan"`
	MetadataExtraction string              `json:"metadata_extraction"`
	ContentPatterns    map[string][]string `json:"content_patterns"`
}

// WorkerConfig holds worker pool configuration
type WorkerConfig struct {
	MaxConcurrentAnalysis int    `json:"max_concurrent_analysis"`
	AnalysisQueueSize     int    `json:"analysis_queue_size"`
	WorkerTimeout         string `json:"worker_timeout"`
	RetryAttempts         int    `json:"retry_attempts"`
	RetryDelay            string `json:"retry_delay"`
	MaxFileSizeMB         int64  `json:"max_file_size_mb"`
	AnalysisTimeout       string `json:"analysis_timeout"`
}

// Config holds the server configuration
type Config struct {
	ServerPort     string         `json:"server_port"`
	StoragePath    string         `json:"storage_path"`
	WorkerPool     WorkerConfig   `json:"worker_pool"`
	AnalysisParams AnalysisParams `json:"analysis_params"`
}

var (
	db     *gorm.DB
	config Config
)

func main() {
	// Load configuration
	if err := loadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	var err error
	db, err = gorm.Open(sqlite.Open("storage.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate the database schema
	if err := db.AutoMigrate(&StoredFile{}, &AnalysisResult{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	// Initialize worker pool before starting the server
	initializeWorkers()

	// Initialize router
	r := gin.Default()
	setupRoutes(r)

	// Start server
	if err := r.Run(":" + config.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func loadConfig() error {
	configFile, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer configFile.Close()

	return json.NewDecoder(configFile).Decode(&config)
}

func setupRoutes(r *gin.Engine) {
	r.GET("/health", healthCheck)
	r.HEAD("/health", healthCheck) // Add support for HEAD requests
	r.POST("/upload_file", handleFileUpload)
	r.GET("/analysis/:file_id", getAnalysisResult)
	r.GET("/files", listFiles)
}
