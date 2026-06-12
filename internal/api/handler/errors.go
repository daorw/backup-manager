package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// respondError writes an appropriate HTTP error response based on the error message.
// It uses simple string matching to determine the HTTP status code:
//   - "not found" → 404
//   - "is required" or "invalid" → 400
//   - everything else → 500
func respondError(c *gin.Context, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": msg})
	case strings.Contains(msg, "is required") || strings.Contains(msg, "invalid"):
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
	}
}
