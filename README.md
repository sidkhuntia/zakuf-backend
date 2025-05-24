# Zakuf Backend - Enhanced Document Conversion Service

A high-performance Go backend service that streams files directly to Gotenberg for document conversion, supporting multiple engines and formats with intelligent fallback processing.

## Features

- **Direct Streaming**: Files stream directly to Gotenberg without local storage
- **Multiple Engines**: LibreOffice, Chromium HTML, Markdown, and URL conversion
- **Smart Fallback**: Local DOCX processing when Gotenberg is unavailable
- **Real-time Health**: Health check endpoint with Gotenberg status
- **Advanced Options**: Engine-specific conversion parameters
- **Zero Storage**: No temporary file storage for uploads (except fallback)

## Supported Conversion Types

### 1. LibreOffice Documents (`libreoffice`)
- **Formats**: DOCX, XLSX, PPTX, ODT, ODS, ODP, DOC, XLS, PPT, RTF
- **Options**: Flatten, merge, landscape, page ranges
- **Fallback**: Local UniOffice processing available

### 2. Chromium HTML (`chromium-html`)
- **Formats**: HTML, HTM files
- **Options**: Paper size, margins, scale, background graphics
- **Fallback**: None (requires Gotenberg)

### 3. Chromium Markdown (`chromium-markdown`)
- **Formats**: MD, Markdown files
- **Options**: Custom HTML templates, paper settings
- **Special**: Supports `{{ toHTML "file.md" }}` template function
- **Fallback**: None (requires Gotenberg)

### 4. Chromium URL (`chromium-url`)
- **Input**: Live web page URLs
- **Options**: Paper size, margins, scale, background graphics
- **Fallback**: None (requires Gotenberg)

## Environment Variables

Create a `.env` file in the root directory:

```env
# UniCloud License Key (for local fallback processing)
UNICLOUD_METERED_KEY=your_unicloud_key_here

# Basic Auth Credentials (optional)
USERNAME=admin
PASSWORD=your_secure_password_here

# Gotenberg Service URL
GOTENBERG_URL=http://localhost:3000

# Server Configuration
PORT=8080
```

## Setup

1. **Install dependencies**
   ```bash
   go mod download
   ```

2. **Set up environment**
   ```bash
   cp .env.example .env
   # Edit .env with your actual values
   ```

3. **Start Gotenberg**
   ```bash
   docker run --rm -p 3000:3000 gotenberg/gotenberg:8
   ```

4. **Run the server**
   ```bash
   go run main.go
   ```

## API Endpoints

### GET /health
Check service and Gotenberg status.

**Response**:
```json
{
  "status": "ok",
  "gotenberg": "available",
  "gotenbergURL": "http://localhost:3000"
}
```

### GET /conversion-types
Get available conversion engines and their capabilities.

**Response**:
```json
{
  "types": [
    {
      "id": "libreoffice",
      "name": "LibreOffice Documents",
      "description": "Convert DOCX, XLSX, PPTX, ODT, ODS, ODP to PDF",
      "supportedFormats": ["docx", "xlsx", "pptx", "odt", "ods", "odp"],
      "options": ["flatten", "merge", "landscape", "nativePageRanges"]
    }
  ]
}
```

### POST /convert
Convert uploaded files using specified engine.

**Request**: Multipart form with:
- `files`: File uploads
- `conversionType`: Engine type (libreoffice, chromium-html, etc.)
- `flatten`: Boolean for form flattening
- `merge`: Boolean for file merging
- `landscape`: Boolean for landscape orientation
- `paperWidth`, `paperHeight`: Paper dimensions (inches)
- `marginTop`, `marginBottom`, `marginLeft`, `marginRight`: Margins (inches)
- `printBackground`: Boolean for background graphics
- `scale`: Scale factor (0.1-2.0)
- `nativePageRanges`: Page ranges (e.g., "1-3,5,7-9")
- `indexHtml`: HTML template for Markdown conversion

**Response**: PDF file download

### POST /convert-url
Convert a web page URL to PDF.

**Request**:
```json
{
  "url": "https://example.com",
  "options": {
    "paperWidth": 8.5,
    "paperHeight": 11,
    "printBackground": true,
    "scale": 1.0
  }
}
```

**Response**: PDF file download

### GET /admin (Basic Auth Required)
Admin endpoint for health checks.

## Architecture

### Direct Streaming Flow
```
Client Upload → Backend → Gotenberg → PDF Response
                   ↓
              Local Fallback (DOCX only)
```

### Key Improvements
1. **No Local Storage**: Files stream directly to Gotenberg
2. **Engine Selection**: Choose optimal conversion engine
3. **Smart Fallback**: Automatic fallback for LibreOffice conversions
4. **Real-time Status**: Live Gotenberg availability checking
5. **Advanced Options**: Engine-specific parameter support

## Gotenberg Integration

### LibreOffice Route
- **Endpoint**: `/forms/libreoffice/convert` or `/forms/libreoffice/merge`
- **Files**: Office documents
- **Options**: `flatten`, `landscape`, `nativePageRanges`

### Chromium HTML Route
- **Endpoint**: `/forms/chromium/convert/html`
- **Files**: HTML files with assets
- **Options**: Paper size, margins, scale, background

### Chromium Markdown Route
- **Endpoint**: `/forms/chromium/convert/markdown`
- **Files**: Markdown files + optional index.html template
- **Special**: Template function `{{ toHTML "file.md" }}`

### Chromium URL Route
- **Endpoint**: `/forms/chromium/convert/url`
- **Input**: URL parameter
- **Options**: Paper size, margins, scale, background

## Error Handling

1. **Gotenberg Unavailable**: Automatic fallback for LibreOffice conversions
2. **Unsupported Format**: Clear error messages with supported formats
3. **Invalid Options**: Parameter validation with helpful feedback
4. **Network Issues**: Timeout handling and retry logic

## Performance

- **Memory Efficient**: No local file storage for uploads
- **Fast Processing**: Direct streaming to Gotenberg
- **Concurrent Safe**: Multiple simultaneous conversions
- **Resource Cleanup**: Automatic temporary file cleanup

## Dependencies

- **Gin**: HTTP web framework
- **UniOffice**: Local DOCX to PDF conversion (fallback)
- **pdfcpu**: PDF manipulation and merging
- **Gotenberg**: Primary document conversion service

## Monitoring

Use the `/health` endpoint to monitor:
- Service availability
- Gotenberg connectivity
- Response times
- Error rates

## License

This project uses UniOffice which requires a license for commercial use. Set `UNICLOUD_METERED_KEY` for fallback processing capabilities. 