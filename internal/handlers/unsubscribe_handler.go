package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) UnsubscribeByToken(c *gin.Context) {
	token := c.Param("token")

	log.WithField("action", "unsubscribe").WithField("token", token).Info("user action")

	if err := uuid.Validate(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
		return
	}

	err := h.repo.Unsubscribe(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed successfully"})
}
