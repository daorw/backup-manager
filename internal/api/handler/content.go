package handler

import (
	"net/http"
	"strconv"

	"backup-manager/internal/service"

	"github.com/gin-gonic/gin"
)

// ContentHandler 处理仓库内容的浏览、预览与保存。
type ContentHandler struct {
	svc *service.ContentService
}

// NewContentHandler 创建内容处理器。
func NewContentHandler(svc *service.ContentService) *ContentHandler {
	return &ContentHandler{svc: svc}
}

// Tree 处理 GET /api/v1/repos/:id/tree?path=&include_hidden=
func (h *ContentHandler) Tree(c *gin.Context) {
	includeHidden := false
	if raw := c.Query("include_hidden"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid include_hidden value"})
			return
		}
		includeHidden = value
	}

	entries, err := h.svc.Tree(c.Param("id"), c.Query("path"), includeHidden)
	if err != nil {
		respondError(c, err)
		return
	}
	if entries == nil {
		entries = []service.ContentEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// Preview 处理 GET /api/v1/repos/:id/preview?path=
func (h *ContentHandler) Preview(c *gin.Context) {
	res, err := h.svc.Preview(c.Param("id"), c.Query("path"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// Save 处理 PUT /api/v1/repos/:id/save
func (h *ContentHandler) Save(c *gin.Context) {
	var req service.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	res, err := h.svc.Save(c.Param("id"), req.Path, req.Content)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

// Changes 处理 GET /api/v1/repos/:id/changes
// 返回 data/ 下未提交的变更，作为「有新变更」的信号。
func (h *ContentHandler) Changes(c *gin.Context) {
	changes, err := h.svc.Changes(c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if changes == nil {
		changes = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"dirty": len(changes) > 0, "changes": changes}})
}
