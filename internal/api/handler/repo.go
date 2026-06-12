package handler

import (
	"net/http"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// RepoHandler handles repository-related HTTP requests.
type RepoHandler struct {
	repoSvc *service.RepoService
}

// NewRepoHandler creates a new RepoHandler.
func NewRepoHandler(repoSvc *service.RepoService) *RepoHandler {
	return &RepoHandler{repoSvc: repoSvc}
}

// repoResponse is the JSON response for a repo with config.
type repoResponse struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Path               string `json:"path"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	LastBackupAt       string `json:"last_backup_at,omitempty"`
	Status             string `json:"status"`
	GitInitialized     bool   `json:"git_initialized"`
	RemoteURL          string `json:"remote_url,omitempty"`
	Branch             string `json:"branch,omitempty"`
	AutoBackup         bool   `json:"auto_backup"`
	AutoBackupInterval string `json:"auto_backup_interval,omitempty"`
	GitUserName        string `json:"git_user_name,omitempty"`
	GitUserEmail       string `json:"git_user_email,omitempty"`
}

// Create handles POST /api/v1/repos
func (h *RepoHandler) Create(c *gin.Context) {
	var req service.CreateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	repo, err := h.repoSvc.Create(&req)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": repo})
}

// List handles GET /api/v1/repos
func (h *RepoHandler) List(c *gin.Context) {
	repos, configs, err := h.repoSvc.List()
	if err != nil {
		respondError(c, err)
		return
	}

	items := make([]repoResponse, 0, len(repos))
	for i, r := range repos {
		gitInit, _ := h.repoSvc.IsGitInitialized(r.ID)
		item := repoResponse{
			ID:             r.ID,
			Name:           r.Name,
			Path:           r.Path,
			CreatedAt:      r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Status:         string(r.Status),
			GitInitialized: gitInit,
		}
		if r.LastBackupAt != nil {
			item.LastBackupAt = r.LastBackupAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if i < len(configs) {
			item.RemoteURL = configs[i].RemoteURL
			item.Branch = configs[i].Branch
			item.AutoBackup = configs[i].AutoBackup
			item.AutoBackupInterval = configs[i].AutoBackupInterval
			item.GitUserName = configs[i].GitUserName
			item.GitUserEmail = configs[i].GitUserEmail
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}

// Get handles GET /api/v1/repos/:id
func (h *RepoHandler) Get(c *gin.Context) {
	id := c.Param("id")
	repo, config, err := h.repoSvc.Get(id)
	if err != nil {
		respondError(c, err)
		return
	}

	gitInit, _ := h.repoSvc.IsGitInitialized(repo.ID)
	detail := repoResponse{
		ID:                 repo.ID,
		Name:               repo.Name,
		Path:               repo.Path,
		CreatedAt:          repo.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:          repo.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Status:             string(repo.Status),
		GitInitialized:     gitInit,
		RemoteURL:          config.RemoteURL,
		Branch:             config.Branch,
		AutoBackup:         config.AutoBackup,
		AutoBackupInterval: config.AutoBackupInterval,
		GitUserName:        config.GitUserName,
		GitUserEmail:       config.GitUserEmail,
	}
	if repo.LastBackupAt != nil {
		detail.LastBackupAt = repo.LastBackupAt.Format("2006-01-02T15:04:05Z07:00")
	}

	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// Delete handles DELETE /api/v1/repos/:id
func (h *RepoHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.repoSvc.Delete(id); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "deleted"})
}

// UpdateConfig handles PUT /api/v1/repos/:id/config
func (h *RepoHandler) UpdateConfig(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.repoSvc.UpdateConfig(id, &req); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "updated"})
}

// GitInit handles POST /api/v1/repos/:id/git-init
func (h *RepoHandler) GitInit(c *gin.Context) {
	id := c.Param("id")
	if err := h.repoSvc.GitInit(id); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "initialized"})
}
