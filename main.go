package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type ConversionRequest struct {
	ConversionType string            `json:"conversionType"` // "libreoffice", "chromium-html", "chromium-url", "chromium-markdown"
	Options        ConversionOptions `json:"options"`
}

type ConversionOptions struct {
	// Common options
	Flatten      bool   `json:"flatten"`
	Merge        bool   `json:"merge"`
	OutputFormat string `json:"outputFormat"`

	// LibreOffice specific
	Landscape        bool   `json:"landscape"`
	NativePageRanges string `json:"nativePageRanges"`

	// Chromium specific
	PaperWidth      float64 `json:"paperWidth"`
	PaperHeight     float64 `json:"paperHeight"`
	MarginTop       float64 `json:"marginTop"`
	MarginBottom    float64 `json:"marginBottom"`
	MarginLeft      float64 `json:"marginLeft"`
	MarginRight     float64 `json:"marginRight"`
	PrintBackground bool    `json:"printBackground"`
	Scale           float64 `json:"scale"`

	// URL conversion specific
	URL string `json:"url"`

	// Markdown specific
	IndexHTML string `json:"indexHtml"` // HTML template for markdown
}

type URLConversionRequest struct {
	URL     string            `json:"url"`
	Options ConversionOptions `json:"options"`
}



func getGotenbergURL() string {
	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		gotenbergURL = "http://localhost:3000"
	}
	return gotenbergURL
}

func proxyToGotenbergDirect(files []*multipart.FileHeader, conversionType string, options ConversionOptions) ([]byte, error) {
	gotenbergURL := getGotenbergURL()

	// Determine endpoint based on conversion type
	var endpoint string
	switch conversionType {
	case "libreoffice":
		if options.Merge {
			endpoint = "/forms/libreoffice/merge"
		} else {
			endpoint = "/forms/libreoffice/convert"
		}
	case "chromium-html":
		endpoint = "/forms/chromium/convert/html"
	case "chromium-markdown":
		endpoint = "/forms/chromium/convert/markdown"
	default:
		return nil, fmt.Errorf("unsupported conversion type: %s", conversionType)
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add files to form
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %v", fileHeader.Filename, err)
		}
		defer file.Close()

		// Create form file field
		part, err := writer.CreateFormFile("files", fileHeader.Filename)
		if err != nil {
			return nil, fmt.Errorf("failed to create form file: %v", err)
		}

		// Copy file content directly from upload
		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("failed to copy file content: %v", err)
		}
	}

	// Add conversion options based on type
	addGotenbergOptions(writer, conversionType, options)

	// Close the writer
	writer.Close()

	// Create request to Gotenberg
	req, err := http.NewRequest("POST", gotenbergURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Gotenberg: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gotenberg returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return result, nil
}

func proxyURLToGotenberg(url string, options ConversionOptions) ([]byte, error) {
	gotenbergURL := getGotenbergURL()
	endpoint := "/forms/chromium/convert/url"

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add URL
	writer.WriteField("url", url)

	// Add conversion options
	addGotenbergOptions(writer, "chromium-url", options)

	// Close the writer
	writer.Close()

	// Create request to Gotenberg
	req, err := http.NewRequest("POST", gotenbergURL+endpoint, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Gotenberg: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Gotenberg returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return result, nil
}

func addGotenbergOptions(writer *multipart.Writer, conversionType string, options ConversionOptions) {
	// Common options
	if options.Flatten {
		writer.WriteField("flatten", "true")
	}

	// LibreOffice specific options
	if conversionType == "libreoffice" {
		if options.Landscape {
			writer.WriteField("landscape", "true")
		}
		if options.NativePageRanges != "" {
			writer.WriteField("nativePageRanges", options.NativePageRanges)
		}
	}

	// Chromium specific options
	if strings.HasPrefix(conversionType, "chromium") {
		if options.PaperWidth > 0 {
			writer.WriteField("paperWidth", fmt.Sprintf("%.2f", options.PaperWidth))
		}
		if options.PaperHeight > 0 {
			writer.WriteField("paperHeight", fmt.Sprintf("%.2f", options.PaperHeight))
		}
		if options.MarginTop > 0 {
			writer.WriteField("marginTop", fmt.Sprintf("%.2f", options.MarginTop))
		}
		if options.MarginBottom > 0 {
			writer.WriteField("marginBottom", fmt.Sprintf("%.2f", options.MarginBottom))
		}
		if options.MarginLeft > 0 {
			writer.WriteField("marginLeft", fmt.Sprintf("%.2f", options.MarginLeft))
		}
		if options.MarginRight > 0 {
			writer.WriteField("marginRight", fmt.Sprintf("%.2f", options.MarginRight))
		}
		if options.PrintBackground {
			writer.WriteField("printBackground", "true")
		}
		if options.Scale > 0 {
			writer.WriteField("scale", fmt.Sprintf("%.2f", options.Scale))
		}
	}
}



func main() {
	godotenv.Load()
	r := gin.Default()

	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")

	if username != "" && password != "" {
		auth := r.Group("/", gin.BasicAuth(gin.Accounts{
			username: password,
		}))

		auth.GET("/admin", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "Admin page"})
		})
	}

	// Configure CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://zakuf.sidkhuntia.in", "http://localhost:5173", "https://www.zakuf.sidkhuntia.in"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Create temp directory for fallback processing only
	if err := os.MkdirAll("./temp", 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		gotenbergURL := getGotenbergURL()

		// Check Gotenberg availability
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(gotenbergURL + "/health")
		gotenbergStatus := "unavailable"
		if err == nil && resp.StatusCode == http.StatusOK {
			gotenbergStatus = "available"
		}
		if resp != nil {
			resp.Body.Close()
		}

		c.JSON(http.StatusOK, gin.H{
			"status":       "ok",
			"gotenberg":    gotenbergStatus,
			"gotenbergURL": gotenbergURL,
		})
	})

	// Direct file conversion endpoint (streams directly to Gotenberg)
	r.POST("/convert", func(c *gin.Context) {
		var conversionReq ConversionRequest

		// Parse JSON from form field
		if conversionTypeStr := c.PostForm("conversionType"); conversionTypeStr != "" {
			conversionReq.ConversionType = conversionTypeStr
		} else {
			conversionReq.ConversionType = "libreoffice" // default
		}

		// Parse options from form fields
		conversionReq.Options.Flatten = c.PostForm("flatten") == "true"
		conversionReq.Options.Merge = c.PostForm("merge") == "true"
		conversionReq.Options.Landscape = c.PostForm("landscape") == "true"
		conversionReq.Options.PrintBackground = c.PostForm("printBackground") == "true"

		// Parse numeric options
		if paperWidth := c.PostForm("paperWidth"); paperWidth != "" {
			fmt.Sscanf(paperWidth, "%f", &conversionReq.Options.PaperWidth)
		}
		if paperHeight := c.PostForm("paperHeight"); paperHeight != "" {
			fmt.Sscanf(paperHeight, "%f", &conversionReq.Options.PaperHeight)
		}
		if marginTop := c.PostForm("marginTop"); marginTop != "" {
			fmt.Sscanf(marginTop, "%f", &conversionReq.Options.MarginTop)
		}
		if marginBottom := c.PostForm("marginBottom"); marginBottom != "" {
			fmt.Sscanf(marginBottom, "%f", &conversionReq.Options.MarginBottom)
		}
		if marginLeft := c.PostForm("marginLeft"); marginLeft != "" {
			fmt.Sscanf(marginLeft, "%f", &conversionReq.Options.MarginLeft)
		}
		if marginRight := c.PostForm("marginRight"); marginRight != "" {
			fmt.Sscanf(marginRight, "%f", &conversionReq.Options.MarginRight)
		}
		if scale := c.PostForm("scale"); scale != "" {
			fmt.Sscanf(scale, "%f", &conversionReq.Options.Scale)
		}

		conversionReq.Options.NativePageRanges = c.PostForm("nativePageRanges")
		conversionReq.Options.IndexHTML = c.PostForm("indexHtml")

		// Get uploaded files
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form: " + err.Error()})
			return
		}

		files := form.File["files"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files uploaded"})
			return
		}

		// Try Gotenberg first
		result, err := proxyToGotenbergDirect(files, conversionReq.ConversionType, conversionReq.Options)
		if err != nil {
			fmt.Printf("Gotenberg failed: %v, falling back to local processing\n", err)

			// Only fallback for LibreOffice conversions
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gotenberg conversion failed: " + err.Error()})
			return
		}

		// Return PDF
		outputFilename := fmt.Sprintf("converted_%s.pdf", time.Now().Format("20060102150405"))
		c.Header("Content-Disposition", "attachment; filename="+outputFilename)
		c.Header("Content-Type", "application/pdf")
		c.Data(http.StatusOK, "application/pdf", result)
	})

	// URL to PDF conversion endpoint
	r.POST("/convert-url", func(c *gin.Context) {
		var urlReq URLConversionRequest
		if err := c.ShouldBindJSON(&urlReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if urlReq.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
			return
		}

		// Convert URL to PDF using Gotenberg
		result, err := proxyURLToGotenberg(urlReq.URL, urlReq.Options)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "URL conversion failed: " + err.Error()})
			return
		}

		// Return PDF
		outputFilename := fmt.Sprintf("url_converted_%s.pdf", time.Now().Format("20060102150405"))
		c.Header("Content-Disposition", "attachment; filename="+outputFilename)
		c.Header("Content-Type", "application/pdf")
		c.Data(http.StatusOK, "application/pdf", result)
	})

	// Get supported conversion types
	r.GET("/conversion-types", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"types": []gin.H{
				{
					"id":               "libreoffice",
					"name":             "LibreOffice Documents",
					"description":      "Convert DOCX, XLSX, PPTX, ODT, ODS, ODP to PDF",
					"supportedFormats": []string{"docx", "xlsx", "pptx", "odt", "ods", "odp", "doc", "xls", "ppt", "rtf"},
					"options":          []string{"flatten", "merge", "landscape", "nativePageRanges"},
				},
				{
					"id":               "chromium-html",
					"name":             "HTML to PDF",
					"description":      "Convert HTML files to PDF using Chromium",
					"supportedFormats": []string{"html", "htm"},
					"options":          []string{"paperWidth", "paperHeight", "margins", "printBackground", "scale"},
				},
				{
					"id":               "chromium-markdown",
					"name":             "Markdown to PDF",
					"description":      "Convert Markdown files to PDF using Chromium",
					"supportedFormats": []string{"md", "markdown"},
					"options":          []string{"indexHtml", "paperWidth", "paperHeight", "margins", "printBackground", "scale"},
				},
				{
					"id":               "chromium-url",
					"name":             "URL to PDF",
					"description":      "Convert web pages to PDF using Chromium",
					"supportedFormats": []string{"url"},
					"options":          []string{"paperWidth", "paperHeight", "margins", "printBackground", "scale"},
				},
			},
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}

