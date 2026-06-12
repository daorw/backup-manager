package handler

import (
	"net/http"

	"backup-manager/internal/model"
	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// SymlinkHandler handles symlink-related HTTP requests.
type SymlinkHandler struct {
	symSvc *service.SymlinkService
}

// NewSymlinkHandler creates a new SymlinkHandler.
func NewSymlinkHandler(symSvc *service.SymlinkService) *SymlinkHandler {
	return &SymlinkHandler{symSvc: symSvc}
}

// symlinkResponse is the JSON response for a symlink.
type symlinkResponse struct {
	ID           string `json:"id"`
	RepoID       string `json:"repo_id"`
	RelativePath string `json:"relative_path"`
	TargetPath   string `json:"target_path"`
	Type         string `json:"type"`
	FileSize     int64  `json:"file_size,omitempty"`
	ModifiedAt   string `json:"modified_at,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func symlinkToResponse(s *model.Symlink) symlinkResponse {
	r := symlinkResponse{
		ID:           s.ID,
		RepoID:       s.RepoID,
		RelativePath: s.RelativePath,
		TargetPath:   s.TargetPath,
		Type:         string(s.Type),
		FileSize:     s.FileSize,
		CreatedAt:    s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if s.ModifiedAt != nil {
		r.ModifiedAt = s.ModifiedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return r
}

// Create handles POST /api/v1/repos/:id/symlinks
func (h *SymlinkHandler) Create(c *gin.Context) {
	repoID := c.Param("id")

	var req service.CreateSymlinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	sym, err := h.symSvc.Create(repoID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": symlinkToResponse(sym)})
}

// List handles GET /api/v1/repos/:id/symlinks
func (h *SymlinkHandler) List(c *gin.Context) {
	repoID := c.Param("id")

	syms, err := h.symSvc.List(repoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]symlinkResponse, 0, len(syms))
	for _, s := range syms {
		items = append(items, symlinkToResponse(s))
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Get handles GET /api/v1/repos/:id/symlinks/:linkId
func (h *SymlinkHandler) Get(c *gin.Context) {
	linkID := c.Param("linkId")

	sym, err := h.symSvc.Get(linkID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": symlinkToResponse(sym)})
}

// Delete handles DELETE /api/v1/repos/:id/symlinks/:linkId
func (h *SymlinkHandler) Delete(c *gin.Context) {
	linkID := c.Param("linkId")

	if err := h.symSvc.Delete(linkID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "deleted"})
}

// UpdateTarget handles PUT /api/v1/repos/:id/symlinks/:linkId
func (h *SymlinkHandler) UpdateTarget(c *gin.Context) {
	linkID := c.Param("linkId")

	var req struct {
		TargetPath string `json:"target_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	sym, err := h.symSvc.UpdateTarget(linkID, req.TargetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": symlinkToResponse(sym)})
}

// BatchImport handles POST /api/v1/repos/:id/symlinks/batch
func (h *SymlinkHandler) BatchImport(c *gin.Context) {
	repoID := c.Param("id")

	var req struct {
		Targets []string `json:"targets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if len(req.Targets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "targets is required"})
		return
	}

	syms, err := h.symSvc.BatchImport(repoID, req.Targets)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]symlinkResponse, 0, len(syms))
	for _, s := range syms {
		items = append(items, symlinkToResponse(s))
	}

	c.JSON(http.StatusCreated, gin.H{"data": items})
}
