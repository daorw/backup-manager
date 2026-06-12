package handler

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"backup-manager/internal/git"
	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// BackupHandler handles backup-related HTTP requests.
type BackupHandler struct {
	backupSvc *service.BackupService
}

// NewBackupHandler creates a new BackupHandler.
func NewBackupHandler(backupSvc *service.BackupService) *BackupHandler {
	return &BackupHandler{backupSvc: backupSvc}
}

// TriggerBackupRequest is the request payload for triggering a backup.
type TriggerBackupRequest struct {
	CommitMessage string `json:"commit_message,omitempty"`
}

// Trigger handles POST /api/v1/repos/:id/backup
func (h *BackupHandler) Trigger(c *gin.Context) {
	repoID := c.Param("id")

	var req TriggerBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if errors.Is(err, io.EOF) {
			// Empty body is fine — use default commit message
			req.CommitMessage = ""
		} else {
			respondError(c, err)
			return
		}
	}

	result, err := h.backupSvc.Trigger(repoID, req.CommitMessage)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// PushRequest is the request payload for push operations.
type PushRequest struct {
	Force bool `json:"force"`
}

// Push handles POST /api/v1/repos/:id/push
func (h *BackupHandler) Push(c *gin.Context) {
	repoID := c.Param("id")

	var req PushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty body is fine — treat as normal push
		req.Force = false
	}

	var opts []git.PushOption
	if req.Force {
		opts = append(opts, git.WithForce())
		log.Printf("[handler] force push requested for repo %s", repoID)
	}

	if err := h.backupSvc.Push(repoID, opts...); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "pushed"})
}

// History handles GET /api/v1/repos/:id/backup/history
func (h *BackupHandler) History(c *gin.Context) {
	repoID := c.Param("id")

	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	entries, err := h.backupSvc.History(repoID, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}

	type commitResponse struct {
		Hash    string `json:"hash"`
		Author  string `json:"author"`
		Email   string `json:"email"`
		Date    string `json:"date"`
		Message string `json:"message"`
	}

	items := make([]commitResponse, 0, len(entries))
	for _, e := range entries {
		items = append(items, commitResponse{
			Hash:    e.Hash,
			Author:  e.Author,
			Email:   e.Email,
			Date:    e.Date,
			Message: e.Message,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}
