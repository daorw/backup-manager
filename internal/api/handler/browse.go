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
