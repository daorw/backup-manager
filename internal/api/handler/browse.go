package handler

import (
	"net/http"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// BrowseHandler handles filesystem browsing requests.
type BrowseHandler struct {
	browserSvc *service.BrowserService
}

// NewBrowseHandler creates a new BrowseHandler.
func NewBrowseHandler(browserSvc *service.BrowserService) *BrowseHandler {
	return &BrowseHandler{browserSvc: browserSvc}
}

// AllowedRoots handles GET /api/v1/browse/allowed-roots
func (h *BrowseHandler) AllowedRoots(c *gin.Context) {
	roots := h.browserSvc.AllowedRoots()
	if roots == nil {
		roots = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": roots})
}

// Browse handles GET /api/v1/browse
func (h *BrowseHandler) Browse(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "."
	}

	entries, err := h.browserSvc.Browse(path)
	if err != nil {
		respondError(c, err)
		return
	}

	if entries == nil {
		entries = []service.BrowseEntry{}
	}

	c.JSON(http.StatusOK, gin.H{"data": entries})
}
