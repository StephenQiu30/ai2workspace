// Package health exposes the identity service's runtime status.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response identifies the responding service and its health.
type Response struct {
	Service string `json:"service" example:"identity"`
	Status  string `json:"status" example:"ok"`
}

// Get reports process health. No external dependency is required at this stage.
//
// @Summary Check identity service health
// @ID getIdentityHealth
// @Tags health
// @Produce json
// @Success 200 {object} Response
// @Router /api/identity/v1/health [get]
func Get(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Service: "identity", Status: "ok"})
}
