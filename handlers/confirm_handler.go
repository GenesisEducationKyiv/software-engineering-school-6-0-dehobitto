// Package handlers provides functions for all API endpoints
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConfirmByToken extracts the token from params, validates it, looks it up in the database, and responds with the result.
func (h *Handler) ConfirmByToken(c *gin.Context) {
	token := c.Param("token")

	if err := uuid.Validate(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
		return
	}

	err := h.repo.ConfirmSubscriptionByToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription confirmed successfully"})
}
