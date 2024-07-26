// main.go
package main

import (
    "log"
    "os"

    "github.com/gin-contrib/sessions"
    "github.com/gin-contrib/sessions/cookie"
    "github.com/gin-gonic/gin"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

var db *gorm.DB

func initDB() {
    var err error
    db, err = gorm.Open(sqlite.Open("depo.db"), &gorm.Config{})
    if err != nil {
        log.Fatal("failed to connect database")
    }

    // Migrate the schema
    db.AutoMigrate(&User{}, &ESPDevice{}, &FileMetadata{})
}

func main() {
    initDB()

    r := gin.Default()

    // Set up session store
    store := cookie.NewStore([]byte(os.Getenv("SECRET_KEY")))
    r.Use(sessions.Sessions("mysession", store))

    // Register routes
    registerRoutes(r)

    r.Run(":6000") // listen and serve on 0.0.0.0:8080
}
