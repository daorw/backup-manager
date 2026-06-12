package handler

import (
	"net/http"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

const maxRollbackWorkers = 5

// RollbackHandler handles rollback-related HTTP requests.
type RollbackHandler struct {
	rollbackSvc *service.RollbackService
	semaphore   chan struct{}
}

// NewRollbackHandler creates a new RollbackHandler.
func NewRollbackHandler(rollbackSvc *service.RollbackService) *RollbackHandler {
	return &RollbackHandler{
		rollbackSvc: rollbackSvc,
		semaphore:   make(chan struct{}, maxRollbackWorkers),
	}
}

// ListFiles handles GET /api/v1/repos/:id/commits/:hash/changed-files
// Returns the list of files changed in a commit, matched against current symlinks.
func (h *RollbackHandler) ListFiles(c *gin.Context) {
	repoID := c.Param("id")
	hash := c.Param("hash")

	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit hash is required"})
		return
	}

	files, err := h.rollbackSvc.ListCommitFiles(repoID, hash)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": files})
}

// Rollback handles POST /api/v1/repos/:id/rollback
// Restores source files to the state they had in the given commit.
func (h *RollbackHandler) Rollback(c *gin.Context) {
	repoID := c.Param("id")

	var req service.RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, err)
		return
	}

	if req.CommitHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit_hash is required"})
		return
	}

	result, err := h.rollbackSvc.Rollback(repoID, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// GetCommitFile handles GET /api/v1/repos/:id/commits/:hash/files?path=...
// Returns the content of a file at a specific commit for preview.
func (h *RollbackHandler) GetCommitFile(c *gin.Context) {
	repoID := c.Param("id")
	hash := c.Param("hash")
	path := c.Query("path")

	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit hash is required"})
		return
	}
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path query parameter is required"})
		return
	}

	// Acquire semaphore slot for concurrency limiting
	select {
	case h.semaphore <- struct{}{}:
		defer func() { <-h.semaphore }()
	default:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many concurrent requests (max 5)"})
		return
	}

	result, err := h.rollbackSvc.GetCommitFile(repoID, hash, path)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// RestoreFile handles POST /api/v1/repos/:id/commits/:hash/restore
// Restores a single file from the commit back to its source location.
func (h *RollbackHandler) RestoreFile(c *gin.Context) {
	repoID := c.Param("id")
	hash := c.Param("hash")

	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit hash is required"})
		return
	}

	var req service.RestoreFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, err)
		return
	}

	result, err := h.rollbackSvc.RestoreFile(repoID, hash, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}
