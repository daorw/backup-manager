package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRespondErrorMapsTargetConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	messages := []string{
		`repo_path "docs/settings.json" overlaps existing entry "docs"`,
		"repo_path already exists in data/: docs/settings.json",
	}

	for _, message := range messages {
		t.Run(message, func(t *testing.T) {
			router := gin.New()
			router.GET("/", func(c *gin.Context) {
				respondError(c, errors.New(message))
			})

			response := performRequest(router, http.MethodGet, "/", "")
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusConflict, response.Body.String())
			}
		})
	}
}
