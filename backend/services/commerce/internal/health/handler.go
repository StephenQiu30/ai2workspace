// Package health exposes the commerce service's runtime status.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response identifies the responding service and its health.
type Response struct {
	Service string `json:"service" example:"commerce"`
	Status  string `json:"status" example:"ok"`
}

// Get reports process health. No external dependency is required at this stage.
//
// @Summary Check commerce service health
// @ID getCommerceHealth
// @Tags health
// @Produce json
// @Success 200 {object} Response
// @Router /api/commerce/health [get]
func Get(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Service: "commerce", Status: "ok"})
}
