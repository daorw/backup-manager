package handler

import (
	"fmt"
	"net/http"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	maxPreviewSize    = 10 * 1024 * 1024 // 10MB
	maxPreviewWorkers = 5
)

// PreviewHandler handles file preview and save requests.
type PreviewHandler struct {
	previewSvc *service.PreviewService
	semaphore  chan struct{}
}

// NewPreviewHandler creates a new PreviewHandler.
func NewPreviewHandler(previewSvc *service.PreviewService) *PreviewHandler {
	return &PreviewHandler{
		previewSvc: previewSvc,
		semaphore:  make(chan struct{}, maxPreviewWorkers),
	}
}

// Preview handles GET /api/v1/repos/:id/preview?path=...
// Reads from the source file (symlink target), not the data/ copy.
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

	result, err := h.previewSvc.Preview(repoID, relPath)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Save handles PUT /api/v1/repos/:id/save
// Saves edited content to the source file and syncs to data/.
func (h *PreviewHandler) Save(c *gin.Context) {
	repoID := c.Param("id")

	var req service.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	if len(req.Content) > maxPreviewSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("content exceeds maximum size of %d bytes", maxPreviewSize),
		})
		return
	}

	// Acquire semaphore slot
	select {
	case h.semaphore <- struct{}{}:
		defer func() { <-h.semaphore }()
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many concurrent requests (max 5)"})
		return
	}

	result, err := h.previewSvc.SaveFile(repoID, req.Path, req.Content)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}
