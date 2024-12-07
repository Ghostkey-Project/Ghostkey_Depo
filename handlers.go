package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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

	// Create unique filename
	filename := filepath.Join(config.StoragePath, header.Filename)

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

	// Start analysis in a goroutine
	go analyzeFile(storedFile)

	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"file_id": storedFile.ID,
	})
}

// getAnalysisResult returns the analysis result for a specific file
func getAnalysisResult(c *gin.Context) {
	fileID := c.Param("file_id")

	var result AnalysisResult
	if err := db.Where("file_id = ?", fileID).First(&result).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Analysis result not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     result.Status,
		"parameters": result.Parameters,
		"results":    result.Results,
		"start_time": result.StartTime,
		"end_time":   result.EndTime,
		"error":      result.Error,
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
	// Create analysis record
	analysis := AnalysisResult{
		FileID:     file.ID,
		Parameters: "",
		Status:     "pending",
		StartTime:  time.Now(),
	}

	// Convert analysis parameters to JSON string
	paramsJSON, err := json.Marshal(config.AnalysisParams)
	if err != nil {
		errStr := err.Error()
		analysis.Status = "failed"
		analysis.Error = &errStr
		if err := db.Create(&analysis).Error; err != nil {
			log.Printf("Failed to create analysis record: %v", err)
		}
		return
	}
	analysis.Parameters = string(paramsJSON)

	// Save initial analysis record
	if err := db.Create(&analysis).Error; err != nil {
		log.Printf("Failed to create analysis record: %v", err)
		return
	}

	// Perform the actual file analysis
	results, err := performAnalysis(file, config.AnalysisParams)
	now := time.Now()
	analysis.EndTime = &now

	if err != nil {
		errStr := err.Error()
		analysis.Status = "failed"
		analysis.Error = &errStr
	} else {
		analysis.Status = "completed"
		resultsJSON, err := json.Marshal(results)
		if err != nil {
			errStr := "Failed to marshal results: " + err.Error()
			analysis.Status = "failed"
			analysis.Error = &errStr
		} else {
			analysis.Results = string(resultsJSON)
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
	results := map[string]interface{}{
		"file_name": file.FileName,
		"file_size": file.FileSize,
		"esp_id":    file.EspID,
		"content_matches": make(map[string][]string),
	}

	// Read file content
	content, err := os.ReadFile(file.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Convert content to string for pattern matching
	fileContent := strings.ToLower(string(content))

	// Scan for content patterns
	contentMatches := make(map[string][]string)
	for patternName, patterns := range params.ContentPatterns {
		matches := []string{}
		for _, pattern := range patterns {
			if strings.Contains(fileContent, strings.ToLower(pattern)) {
				matches = append(matches, pattern)
			}
		}
		if len(matches) > 0 {
			contentMatches[patternName] = matches
		}
	}

	results["content_matches"] = contentMatches
	results["scan_timestamp"] = time.Now().UTC()

	return results, nil
} 