package handler

import (
	"net/http"
	"strconv"

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

// Home handles GET /api/v1/browse/home — the default starting directory.
func (h *BrowseHandler) Home(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.browserSvc.Home()})
}

// Browse handles GET /api/v1/browse
// Query params:
//   - path: directory to list (default: home directory; `~` is expanded)
//   - include_hidden: "true" to include dot-files/dot-directories
func (h *BrowseHandler) Browse(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = h.browserSvc.Home()
	}

	includeHidden := false
	if raw := c.Query("include_hidden"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			includeHidden = v
		}
	}

	entries, err := h.browserSvc.Browse(path, includeHidden)
	if err != nil {
		respondError(c, err)
		return
	}

	if entries == nil {
		entries = []service.BrowseEntry{}
	}

	c.JSON(http.StatusOK, gin.H{"data": entries})
}
