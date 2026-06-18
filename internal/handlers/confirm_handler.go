package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"subber/internal/logger"
)

func (h *Handler) ConfirmByToken(c *gin.Context) {
	token := c.Param("token")

	if err := uuid.Validate(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
		return
	}

	logger.WithRequestID(h.log, c.Request.Context()).WithField("action", "confirm").Info("user action")

	err := h.repo.ConfirmSubscriptionByToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription confirmed successfully"})
}
