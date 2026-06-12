package handler

import (
	"net/http"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// RollbackHandler handles rollback-related HTTP requests.
type RollbackHandler struct {
	rollbackSvc *service.RollbackService
}

// NewRollbackHandler creates a new RollbackHandler.
func NewRollbackHandler(rollbackSvc *service.RollbackService) *RollbackHandler {
	return &RollbackHandler{rollbackSvc: rollbackSvc}
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.CommitHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commit_hash is required"})
		return
	}

	result, err := h.rollbackSvc.Rollback(repoID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}
