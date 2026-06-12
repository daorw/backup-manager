package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"backup-manager/internal/service"
	"backup-manager/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	maxPreviewSize    = 10 * 1024 * 1024 // 10MB
	maxPreviewWorkers = 5
)

// PreviewHandler handles file preview requests.
type PreviewHandler struct {
	repoSvc  *service.RepoService
	semaphore chan struct{}
}

// NewPreviewHandler creates a new PreviewHandler.
func NewPreviewHandler(repoSvc *service.RepoService) *PreviewHandler {
	return &PreviewHandler{
		repoSvc:   repoSvc,
		semaphore: make(chan struct{}, maxPreviewWorkers),
	}
}

// previewResponse is the JSON response for a file preview.
type previewResponse struct {
	Content  string `json:"content,omitempty"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
	Text     bool   `json:"text"`
	Truncated bool  `json:"truncated,omitempty"`
}

// Preview handles GET /api/v1/repos/:id/preview
func (h *PreviewHandler) Preview(c *gin.Context) {
	repoID := c.Param("id")
	relPath := c.Query("path")

	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path query parameter is required"})
		return
	}

	// Acquire semaphore slot
	select {
	case h.semaphore <- struct{}{}:
		defer func() { <-h.semaphore }()
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many concurrent preview requests (max 5)"})
		return
	}

	repo, _, err := h.repoSvc.Get(repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}

	// Use SafeResolveFile to securely resolve the path within data/ directory
	dataDir := repo.Path + string(os.PathSeparator) + "data"
	absFilePath, err := util.SafeResolveFile(dataDir, relPath)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "file not allowed or not found"})
		return
	}

	info, err := os.Stat(absFilePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot preview a directory"})
		return
	}

	// Check file size limit
	if info.Size() > maxPreviewSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("file too large for preview (max %d bytes)", maxPreviewSize),
		})
		return
	}

	// Detect MIME type
	mimeType, err := detectMIME(absFilePath)
	if err != nil {
		mimeType = "application/octet-stream"
	}

	isText := isTextMIME(mimeType)

	resp := previewResponse{
		MimeType: mimeType,
		Size:     info.Size(),
		Text:     isText,
	}

	if isText {
		content, truncated, err := readTextFile(absFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file: " + err.Error()})
			return
		}
		resp.Content = content
		resp.Truncated = truncated
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// detectMIME reads the first bytes of a file and returns its MIME type.
func detectMIME(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]

	// Use the full 512-byte buffer for detection
	mimeType := http.DetectContentType(buf)
	return mimeType, nil
}

// isTextMIME checks if a MIME type indicates a text-based file.
func isTextMIME(mimeType string) bool {
	// Common text MIME types
	textTypes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/x-yaml",
		"application/x-sh",
		"application/x-python",
		"application/x-go",
		"application/x-ruby",
		"application/x-perl",
		"application/x-php",
		"application/javascript",
		"application/x-javascript",
		"application/x-www-form-urlencoded",
	}

	for _, t := range textTypes {
		if strings.HasPrefix(mimeType, t) {
			return true
		}
	}

	return false
}

// readTextFile reads the content of a text file, limiting to maxPreviewSize.
func readTextFile(filePath string) (string, bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	// Read up to maxPreviewSize + 1 to detect truncation
	buf := make([]byte, maxPreviewSize+1)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", false, err
	}

	truncated := n > maxPreviewSize
	if n > maxPreviewSize {
		n = maxPreviewSize
	}

	return string(buf[:n]), truncated, nil
}
