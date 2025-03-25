package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var analysisQueue chan StoredFile

// Modify initializeWorkers to use config values
func initializeWorkers() {
	analysisQueue = make(chan StoredFile, config.WorkerPool.AnalysisQueueSize)

	for i := 0; i < config.WorkerPool.MaxConcurrentAnalysis; i++ {
		go analysisWorker()
	}
}

// healthCheck responds with the server status
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// Worker function that processes files from the queue
func analysisWorker() {
	// Convert timeout string to duration
	timeout, err := time.ParseDuration(config.WorkerPool.WorkerTimeout)
	if err != nil {
		log.Printf("Invalid worker timeout: %v, using default of 5m", err)
		timeout = 5 * time.Minute
	}

	for file := range analysisQueue {
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		// Create done channel for the analysis
		done := make(chan bool)

		// Start analysis in goroutine
		go func() {
			analyzeFile(file)
			done <- true
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			// Analysis completed successfully
		case <-ctx.Done():
			// Analysis timed out
			log.Printf("Analysis of file %s (ID: %d) timed out", file.FileName, file.ID)
			// Update analysis record with timeout error
			updateAnalysisTimeout(file)
		}

		cancel() // Clean up context
	}
}

func updateAnalysisTimeout(file StoredFile) {
	var analysis AnalysisResult
	if err := db.Where("file_id = ?", file.ID).First(&analysis).Error; err == nil {
		errStr := "Analysis timed out"
		analysis.Status = "failed"
		analysis.Error = &errStr
		db.Save(&analysis)
	}
}

// handleFileUpload handles incoming file uploads from the main server
func handleFileUpload(c *gin.Context) {
	// Get file from form data
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file received"})
		return
	}
	defer file.Close()

	// Get metadata from form
	espID := c.PostForm("esp_id")
	deliveryKey := c.PostForm("delivery_key")
	encryptionPassword := c.PostForm("encryption_password")

	if espID == "" || deliveryKey == "" || encryptionPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required metadata"})
		return
	}

	// Create directory for this delivery key if it doesn't exist
	deliveryKeyPath := filepath.Join(config.StoragePath, deliveryKey)
	if err := os.MkdirAll(deliveryKeyPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create delivery directory"})
		return
	}

	// Create unique filename within the delivery key directory
	filename := filepath.Join(deliveryKeyPath, header.Filename)

	// If file already exists, append timestamp to make it unique
	if _, err := os.Stat(filename); err == nil {
		ext := filepath.Ext(header.Filename)
		base := strings.TrimSuffix(header.Filename, ext)
		timestamp := time.Now().Format("20060102150405")
		filename = filepath.Join(deliveryKeyPath, fmt.Sprintf("%s_%s%s", base, timestamp, ext))
	}

	// Create the file
	out, err := os.Create(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
		return
	}
	defer out.Close()

	// Copy the file data
	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Get file info for size
	fileInfo, err := out.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file info"})
		return
	}

	// Create database record
	storedFile := StoredFile{
		FileName:           header.Filename,
		FilePath:           filename,
		EspID:              espID,
		DeliveryKey:        deliveryKey,
		EncryptionPassword: encryptionPassword,
		FileSize:           fileInfo.Size(),
		UploadTime:         time.Now(),
	}

	if err := db.Create(&storedFile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	// Queue file for analysis if possible
	var queuedForAnalysis bool
	select {
	case analysisQueue <- storedFile:
		queuedForAnalysis = true
	default:
		queuedForAnalysis = false
		log.Printf("Warning: Analysis queue is full. File %s (ID: %d) will be analyzed later",
			storedFile.FileName, storedFile.ID)
	}

	// Always return the file ID in the response
	c.JSON(http.StatusOK, gin.H{
		"message":             "File uploaded successfully",
		"file_id":             storedFile.ID,
		"queued_for_analysis": queuedForAnalysis,
	})
}

// getAnalysisResult returns the analysis result for a specific file
func getAnalysisResult(c *gin.Context) {
	fileID := c.Param("file_id")

	var result AnalysisResult
	// Include the StoredFile in the query using Preload
	if err := db.Preload("StoredFile").Where("file_id = ?", fileID).First(&result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Analysis result not found"})
		return
	}

	// Parse the JSON strings into maps for each category
	var basicInfo, espInfo, metadata, analysis map[string]interface{}
	if result.BasicInfo != "" {
		json.Unmarshal([]byte(result.BasicInfo), &basicInfo)
	}
	if result.EspInfo != "" {
		json.Unmarshal([]byte(result.EspInfo), &espInfo)
	}
	if result.Metadata != "" {
		json.Unmarshal([]byte(result.Metadata), &metadata)
	}
	if result.Analysis != "" {
		json.Unmarshal([]byte(result.Analysis), &analysis)
	}

	c.JSON(http.StatusOK, gin.H{
		"filename":         result.FileName,
		"status":           result.Status,
		"parameters":       result.Parameters,
		"basic_info":       basicInfo,
		"esp_info":         espInfo,
		"metadata":         metadata,
		"content_analysis": analysis,
		"start_time":       result.StartTime,
		"end_time":         result.EndTime,
		"error":            result.Error,
	})
}

// listFiles returns a list of all stored files and their analysis status
func listFiles(c *gin.Context) {
	var files []StoredFile
	if err := db.Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

// analyzeFile performs the analysis of a stored file based on configuration parameters
func analyzeFile(file StoredFile) {
	analysis := AnalysisResult{
		FileID:    file.ID,
		FileName:  file.FileName,
		Status:    "pending",
		StartTime: time.Now(),
	}

	// Convert analysis parameters to JSON string
	paramsJSON, err := json.Marshal(config.AnalysisParams)
	if err != nil {
		errStr := err.Error()
		analysis.Status = "failed"
		analysis.Error = &errStr
		db.Create(&analysis)
		return
	}
	analysis.Parameters = string(paramsJSON)

	// Save initial analysis record
	if err := db.Create(&analysis).Error; err != nil {
		log.Printf("Failed to create analysis record: %v", err)
		return
	}

	// Perform the analysis
	results, err := performAnalysis(file, config.AnalysisParams)
	now := time.Now()
	analysis.EndTime = &now

	if err != nil {
		errStr := err.Error()
		analysis.Status = "failed"
		analysis.Error = &errStr
	} else {
		analysis.Status = "completed"

		// Marshal each section separately
		if basicInfoJSON, err := json.Marshal(results["basic_info"]); err == nil {
			analysis.BasicInfo = string(basicInfoJSON)
		}
		if espInfoJSON, err := json.Marshal(results["esp_info"]); err == nil {
			analysis.EspInfo = string(espInfoJSON)
		}
		if metadataJSON, err := json.Marshal(results["metadata"]); err == nil {
			analysis.Metadata = string(metadataJSON)
		}
		if analysisJSON, err := json.Marshal(results["content_analysis"]); err == nil {
			analysis.Analysis = string(analysisJSON)
		}
	}

	// Update analysis record
	if err := db.Save(&analysis).Error; err != nil {
		log.Printf("Failed to update analysis record: %v", err)
		return
	}

	// Update file's analyzed status
	file.Analyzed = true
	if err := db.Save(&file).Error; err != nil {
		log.Printf("Failed to update file status: %v", err)
	}
}

// performAnalysis implements the actual file analysis logic based on parameters
func performAnalysis(file StoredFile, params AnalysisParams) (map[string]interface{}, error) {
	// Get file info for basic metadata
	// check fileInfo, err := os.Stat(file.FilePath)
	// check if err != nil {
	// check 	return nil, fmt.Errorf("failed to get file info: %v", err)
	// check }

	// Get file extension
	fileExt := strings.ToLower(filepath.Ext(file.FileName))

	// Run exiftool to get metadata
	cmd := exec.Command("exiftool", "-json", file.FilePath)
	output, err := cmd.Output()

	var metadata []map[string]interface{}
	if err != nil {
		log.Printf("Warning: exiftool failed: %v", err)
		metadata = []map[string]interface{}{
			{
				"Error": "Failed to extract metadata: " + err.Error(),
			},
		}
	} else {
		if err := json.Unmarshal(output, &metadata); err != nil {
			return nil, fmt.Errorf("failed to parse exiftool output: %v", err)
		}
	}

	// Initialize metadata map
	var metadataMap map[string]interface{}
	if len(metadata) > 0 {
		metadataMap = metadata[0]
	} else {
		metadataMap = map[string]interface{}{}
	}

	// Create structured results in separate categories
	basicInfo := map[string]interface{}{
		"name":            file.FileName,
		"collection_date": file.UploadTime.Format(time.RFC3339),
		"file_type":       fileExt,
		"size": map[string]interface{}{
			"bytes":          file.FileSize,
			"human_readable": humanReadableSize(file.FileSize),
		},
	}

	espInfo := map[string]interface{}{
		"esp_id":       file.EspID,
		"delivery_key": file.DeliveryKey,
		"is_encrypted": true,
	}

	contentAnalysis := map[string]interface{}{
		"patterns_found": false,
		"matches":        make(map[string][]string),
	}

	// Read file content and perform pattern matching
	content, err := os.ReadFile(file.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	fileContent := strings.ToLower(string(content))
	contentMatches := make(map[string][]string)
	patternsFound := false

	for patternName, patterns := range params.ContentPatterns {
		matches := []string{}
		for _, pattern := range patterns {
			if strings.Contains(fileContent, strings.ToLower(pattern)) {
				matches = append(matches, pattern)
			}
		}
		if len(matches) > 0 {
			contentMatches[patternName] = matches
			patternsFound = true
		}
	}

	contentAnalysis["patterns_found"] = patternsFound
	contentAnalysis["matches"] = contentMatches

	// Combine all results
	results := map[string]interface{}{
		"basic_info":       basicInfo,
		"esp_info":         espInfo,
		"metadata":         metadataMap,
		"content_analysis": contentAnalysis,
		"scan_timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	return results, nil
}

// Add this helper function to convert bytes to human readable format
func humanReadableSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
