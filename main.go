package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	officel "github.com/unidoc/unioffice/common/license"
	"github.com/unidoc/unioffice/document"
	"github.com/unidoc/unioffice/document/convert"
	pdfl "github.com/unidoc/unipdf/v3/common/license"
)

type FileOrder struct {
	Files []string `json:"files"`
}

func setLicenseKey() error {
	apiKey := os.Getenv("UNICLOUD_METERED_KEY")
	if apiKey == "" {
		return fmt.Errorf("Missing UNICLOUD_METERED_KEY environment variable")
	}

	err := officel.SetMeteredKey(apiKey)
	if err != nil {
		return err
	}

	err = pdfl.SetMeteredKey(apiKey)
	if err != nil {
		return err
	}

	return nil
}

func main() {

	godotenv.Load()

	err := setLicenseKey()
	if err != nil {
		fmt.Printf("Failed to load license: %v\n", err)
		os.Exit(1)
	}

	r := gin.Default()

	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	auth := r.Group("/", gin.BasicAuth(gin.Accounts{
		username: password,
	}))

	auth.GET("/admin", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Admin page"})
	})

	// Configure CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Allow all origins
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Create temp directory for uploads
	if err := os.MkdirAll("./uploads", 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// Endpoint to handle file uploads
	r.POST("/upload", func(c *gin.Context) {
		// Multipart form
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sessionID := uuid.New().String()
		sessionDir := filepath.Join("./uploads", sessionID)

		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session directory"})
			return
		}

		// Save uploaded files
		files := form.File["files"]
		savedFiles := make([]string, 0, len(files))

		for i, file := range files {
			filename := fmt.Sprintf("%d_%s", i, file.Filename)
			dst := filepath.Join(sessionDir, filename)

			if err := c.SaveUploadedFile(file, dst); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
				return
			}

			savedFiles = append(savedFiles, filename)
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Files uploaded successfully",
			"sessionId": sessionID,
			"files":     savedFiles,
		})
	})

	// Process files endpoint
	r.POST("/process", func(c *gin.Context) {
		var fileOrder FileOrder
		if err := c.ShouldBindJSON(&fileOrder); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		sessionID := c.Query("sessionId")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID is required"})
			return
		}

		sessionDir := filepath.Join("./uploads", sessionID)
		fmt.Printf("SESSION DIR: %s\n", sessionDir)
		if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
			return
		}

		// Process files based on order
		pdfFiles := make([]string, 0, len(fileOrder.Files))

		for i, filename := range fileOrder.Files {
			filename = fmt.Sprintf("%d_%s", i, filename)
			filePath := filepath.Join(sessionDir, filename)
			ext := strings.ToLower(filepath.Ext(filePath))

			if ext == ".docx" {
				// Convert DOCX to PDF
				pdfPath := filePath + ".pdf"
				if err := convertDocxToPdf(filePath, pdfPath); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to convert DOCX to PDF"})
					return
				}
				pdfFiles = append(pdfFiles, pdfPath)
			} else if ext == ".pdf" {
				pdfFiles = append(pdfFiles, filePath)
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported file format: " + ext})
				return
			}
		}

		outputFilename := "merged_" + time.Now().Format("20060102150405") + ".pdf"
		outputPath := filepath.Join(sessionDir, outputFilename)

		// If only one file, just use it as the result
		if len(pdfFiles) == 1 {
			// Copy the single PDF to the output path
			fmt.Printf(pdfFiles[0])
			src, err := os.Open(filepath.Clean(pdfFiles[0]))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file", "message": err.Error()})
				return
			}
			defer src.Close()

			dst, err := os.Create(outputPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create output file"})
				return
			}
			defer dst.Close()

			if _, err := io.Copy(dst, src); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy file"})
				return
			}
		} else if len(pdfFiles) > 1 {
			// Merge multiple PDFs
			if err := api.MergeCreateFile(pdfFiles, outputPath, false, nil); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to merge PDFs"})
				return
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files to process"})
			return
		}

		// Serve the file
		c.Header("Content-Disposition", "attachment; filename="+outputFilename)
		c.Header("Content-Type", "application/pdf")
		c.File(outputPath)

		// Schedule cleanup
		go func() {
			time.Sleep(1 * time.Hour)
			os.RemoveAll(sessionDir)
		}()
	})

	r.Run(":8080")
}

// Helper function to convert DOCX to PDF using LibreOffice
func convertDocxToPdf(docxPath, pdfPath string) error {

	doc, err := document.Open(docxPath)
	if err != nil {
		return err
	}
	defer doc.Close()
	c := convert.ConvertToPdf(doc)

	err = c.WriteToFile(pdfPath)
	if err != nil {
		return err
	}

	return nil

}
