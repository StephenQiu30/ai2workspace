// Package health exposes the learning service's runtime status.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response identifies the responding service and its health.
type Response struct {
	Service string `json:"service" example:"learning"`
	Status  string `json:"status" example:"ok"`
}

// Get reports process health. No external dependency is required at this stage.
//
// @Summary Check learning service health
// @ID getLearningHealth
// @Tags health
// @Produce json
// @Success 200 {object} Response
// @Router /api/learning/health [get]
func Get(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Service: "learning", Status: "ok"})
}
