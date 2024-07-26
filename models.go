// models.go
package main

import (
    "gorm.io/gorm"
    "golang.org/x/crypto/bcrypt"
)

type User struct {
    gorm.Model
    Username string `gorm:"unique"`
    Password string
}

func (u *User) SetPassword(password string) error {
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    u.Password = string(hashedPassword)
    return nil
}

func (u *User) CheckPassword(password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
    return err == nil
}

type ESPDevice struct {
    gorm.Model
    EspID        string `gorm:"unique"`
    EspSecretKey string
}

type FileMetadata struct {
    gorm.Model
    FileName           string
    OriginalFileName   string
    FilePath           string
    EspID              string
    DeliveryKey        string
    EncryptionPassword string
}
