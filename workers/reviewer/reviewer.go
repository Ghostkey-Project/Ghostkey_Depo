// workers/reviewer/reviewer.go
package reviewer

import (
    "fmt"
    "os"
    "io"
    "strings"
    "path/filepath"
    "gorm.io/gorm"
    "Ghostkey_Depo/models" // Update to the correct import path
)

var keywords = []string{"confidential", "secret", "important"}
var mediaExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".mp4", ".mov"}

const batchSize = 1000

func ReviewFiles(db *gorm.DB) {
    var files []models.FileMetadata
    db.Where("review_status = ?", "").Limit(batchSize).Find(&files)

    for _, file := range files {
        content, err := readFile(file.FilePath)
        if err != nil {
            fmt.Printf("Failed to read file: %s\n", file.FilePath)
            continue
        }

        fileContent := string(content)
        category := categorizeFile(fileContent, file.FileName)

        file.ReviewStatus = category
        db.Save(&file)
        moveFileToCategory(file.FilePath, category)
    }
}

func readFile(filePath string) ([]byte, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    stat, err := file.Stat()
    if err != nil {
        return nil, err
    }

    data := make([]byte, stat.Size())
    _, err = io.ReadFull(file, data)
    if err != nil {
        return nil, err
    }

    return data, nil
}

func categorizeFile(content, fileName string) string {
    for _, keyword := range keywords {
        if strings.Contains(content, keyword) {
            return "need_review"
        }
    }

    ext := strings.ToLower(filepath.Ext(fileName))
    for _, mediaExt := range mediaExtensions {
        if ext == mediaExt {
            return "media"
        }
    }

    if strings.Contains(content, "important") {
        return "important"
    }

    return "trash"
}

func moveFileToCategory(filePath, category string) {
    categoryDir := filepath.Join("storage", category)
    if _, err := os.Stat(categoryDir); os.IsNotExist(err) {
        os.Mkdir(categoryDir, 0755)
    }

    newFilePath := filepath.Join(categoryDir, filepath.Base(filePath))
    os.Rename(filePath, newFilePath)
}
