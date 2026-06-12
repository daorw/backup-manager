package handler

import (
	"net/http"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles Git authentication-related HTTP requests.
type AuthHandler struct {
	authSvc *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Get handles GET /api/v1/repos/:id/auth
func (h *AuthHandler) Get(c *gin.Context) {
	repoID := c.Param("id")

	auth, err := h.authSvc.Get(repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": auth})
}

// Set handles PUT /api/v1/repos/:id/auth
func (h *AuthHandler) Set(c *gin.Context) {
	repoID := c.Param("id")

	var req service.SetAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if err := h.authSvc.Set(repoID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "updated"})
}

// Clear handles DELETE /api/v1/repos/:id/auth
func (h *AuthHandler) Clear(c *gin.Context) {
	repoID := c.Param("id")

	if err := h.authSvc.Clear(repoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "cleared"})
}
