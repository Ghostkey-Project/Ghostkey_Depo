// routes.go
package main

import (
    "net/http"
    "io"
    "os"
    "fmt"
    "path/filepath"
    "sync"
    "github.com/gin-contrib/sessions"
    "github.com/gin-gonic/gin"
)

func registerRoutes(r *gin.Engine) {
    // User routes
    r.POST("/register_user", registerUser)
    r.POST("/login", login)
    r.POST("/logout", logout)

    // Device routes
    r.POST("/register_device", registerDevice) 

    // File routes
    r.POST("/upload_file", uploadFile)
    r.GET("/get_file/:fileName", getFile)
}

func registerUser(c *gin.Context) {
    secretKey := c.PostForm("secret_key")
    expectedSecretKey := os.Getenv("SECRET_KEY")

    if secretKey != expectedSecretKey {
        c.JSON(http.StatusForbidden, gin.H{"message": "Invalid secret key"})
        return
    }

    username := c.PostForm("username")
    password := c.PostForm("password")
    if username == "" || password == "" {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Username and password are required"})
        return
    }

    var user User
    if err := db.Where("username = ?", username).First(&user).Error; err == nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Username already exists"})
        return
    }

    newUser := User{Username: username}
    if err := newUser.SetPassword(password); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to set password"})
        return
    }

    if err := db.Create(&newUser).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to register user"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}

func login(c *gin.Context) {
    username := c.PostForm("username")
    password := c.PostForm("password")
    if username == "" || password == "" {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Username and password are required"})
        return
    }

    var user User
    if err := db.Where("username = ?", username).First(&user).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid username or password"})
        return
    }

    if !user.CheckPassword(password) {
        c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid username or password"})
        return
    }

    session := sessions.Default(c)
    session.Set("user_id", user.ID)
    session.Save()

    c.JSON(http.StatusOK, gin.H{"message": "Logged in successfully"})
}

func logout(c *gin.Context) {
    session := sessions.Default(c)
    session.Clear()
    session.Save()

    c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func registerDevice(c *gin.Context) {
    espID := c.PostForm("esp_id")
    espSecretKey := c.PostForm("esp_secret_key")

    if espID == "" || espSecretKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"message": "ESP ID and secret key are required"})
        return
    }

    var device ESPDevice
    if err := db.Where("esp_id = ?", espID).First(&device).Error; err == nil {
        c.JSON(http.StatusBadRequest, gin.H{"message": "ESP ID already exists"})
        return
    }

    newDevice := ESPDevice{EspID: espID, EspSecretKey: espSecretKey}
    if err := db.Create(&newDevice).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to register ESP32"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "ESP32 registered successfully", "esp_id": espID})
}

var (
    idCounter = 1
    idMutex   sync.Mutex
)

func getNextID() int {
    idMutex.Lock()
    defer idMutex.Unlock()
    idCounter++
    return idCounter
}

func uploadFile(c *gin.Context) {
    espID := c.PostForm("esp_id")
    deliveryKey := c.PostForm("delivery_key")
    encryptionPassword := c.PostForm("encryption_password")

    if espID == "" || deliveryKey == "" || encryptionPassword == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "ESP ID, delivery key, and encryption password are required"})
        return
    }

    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "File upload failed", "details": err.Error()})
        return
    }
    defer file.Close()

    uniqueID := getNextID()
    fileName := fmt.Sprintf("%d-%s", uniqueID, header.Filename)
    outputDir := "cargo_files"
    if _, err := os.Stat(outputDir); os.IsNotExist(err) {
        err := os.Mkdir(outputDir, 0755)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory", "details": err.Error()})
            return
        }
    }

    outputPath := filepath.Join(outputDir, fileName)
    out, err := os.Create(outputPath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file", "details": err.Error()})
        return
    }
    defer out.Close()

    if _, err := io.Copy(out, file); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file", "details": err.Error()})
        return
    }

    fileMetadata := FileMetadata{
        FileName:           fileName,
        OriginalFileName:   header.Filename,
        FilePath:           outputPath,
        EspID:              espID,
        DeliveryKey:        deliveryKey,
        EncryptionPassword: encryptionPassword,
    }
    if err := db.Create(&fileMetadata).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata", "details": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "File delivered successfully"})
}

func getFile(c *gin.Context) {
    fileName := c.Param("fileName")

    var fileMetadata FileMetadata
    if err := db.Where("file_name = ?", fileName).First(&fileMetadata).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"message": "File not found"})
        return
    }

    c.File(fileMetadata.FilePath)
}
