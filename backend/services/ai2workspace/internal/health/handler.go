// Package health exposes the ai2workspace service's runtime status.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response identifies the responding service and its health.
type Response struct {
	Service string `json:"service" example:"ai2workspace"`
	Status  string `json:"status" example:"ok"`
}

// Get reports process health. No external dependency is required at this stage.
//
// @Summary Check ai2workspace service health
// @ID getServiceHealth
// @Tags health
// @Produce json
// @Success 200 {object} Response
// @Router /api/ai2workspace/v1/health [get]
func Get(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Service: "ai2workspace", Status: "ok"})
}
