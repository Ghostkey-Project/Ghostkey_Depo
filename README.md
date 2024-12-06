# Storage Server

This server works in conjunction with the main Ghostkey server to store and analyze files collected from mailers.

## Features

- Receives files from the main server
- Stores files securely with metadata
- Performs configurable file analysis
- Provides API endpoints for file and analysis management
- Stores analysis results in SQLite database

## Setup

1. Install Go (1.16 or later)
2. Install dependencies:
   ```bash
   go mod init storage_server
   go get github.com/gin-gonic/gin
   go get gorm.io/gorm
   go get gorm.io/driver/sqlite
   ```

3. Configure the server:
   - Edit `config.json` to set:
     - Server port
     - Storage path
     - Analysis parameters

4. Run the server:
   ```bash
   go run .
   ```

## API Endpoints

### POST /upload_file
Receives files from the main server with metadata.

Required form fields:
- `file`: The file to upload
- `esp_id`: ESP device ID
- `delivery_key`: Delivery key
- `encryption_password`: Encryption password

### GET /analysis/:file_id
Retrieves analysis results for a specific file.

### GET /files
Lists all stored files and their analysis status.

## Configuration

The `config.json` file contains:
- `server_port`: Port number for the server
- `storage_path`: Path to store uploaded files
- `analysis_params`: Parameters for file analysis
  - `max_file_size`: Maximum allowed file size
  - `allowed_extensions`: List of allowed file extensions
  - `scan_timeout`: Timeout for analysis in seconds
  - `content_check`: Enable content analysis
  - `virus_scan`: Enable virus scanning
  - `metadata_extraction`: Enable metadata extraction

## Database Schema

### StoredFile
- ID (uint)
- FileName (string)
- FilePath (string)
- EspID (string)
- DeliveryKey (string)
- EncryptionPassword (string)
- FileSize (int64)
- UploadTime (time.Time)
- Analyzed (bool)

### AnalysisResult
- ID (uint)
- FileID (uint)
- Parameters (json)
- Results (json)
- Status (string)
- StartTime (time.Time)
- EndTime (time.Time)
- Error (string) 