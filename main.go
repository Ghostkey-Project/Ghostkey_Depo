// main.go
package main

import (
    "log"
    "os"
    "time"
    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/cookie"
    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "Ghostkey_Depo/models"         // Update to the correct import path
    "Ghostkey_Depo/workers/reviewer" // Update to the correct import path
)

var db *gorm.DB

func initDB() {
    var err error
    db, err = gorm.Open(sqlite.Open("depo.db"), &gorm.Config{})
    if err != nil {
        log.Fatal("failed to connect database")
    }

    // Migrate the schema
    db.AutoMigrate(&models.User{}, &models.ESPDevice{}, &models.FileMetadata{})
}

func main() {
    initDB()

    r := gin.Default()

    // Set up session store
    store := cookie.NewStore([]byte(os.Getenv("SECRET_KEY")))
    r.Use(sessions.Sessions("mysession", store))

    // Register routes
    registerRoutes(r)

    // Toggle file review process
    if os.Getenv("ENABLE_REVIEWER") == "true" {
        go startReviewer()
    }

    r.Run(":6000") // listen and serve on 0.0.0.0:6000
}

func startReviewer() {
    ticker := time.NewTicker(1 * time.Hour) // Adjust the interval as needed
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            reviewer.ReviewFiles(db)
        }
    }
}
