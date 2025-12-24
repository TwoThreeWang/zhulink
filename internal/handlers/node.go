package handlers

import (
	"net/http"
	"zhulink/internal/db"
	"zhulink/internal/models"

	"github.com/gin-gonic/gin"
)

type NodeHandler struct{}

func NewNodeHandler() *NodeHandler {
	return &NodeHandler{}
}

// ListNodes 展示所有节点列表
func (h *NodeHandler) ListNodes(c *gin.Context) {
	var nodes []models.Node
	db.DB.Order("id ASC").Find(&nodes)

	Render(c, http.StatusOK, "node/list.html", gin.H{
		"Nodes":  nodes,
		"Title":  "节点",
		"Active": "nodes",
	})
}
