package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"subber/internal/logger"
	"subber/internal/validators"
)

func (h *Handler) GetSubscriptions(c *gin.Context) {
	email := c.Query("email")

	if email == "" || !validators.IsValidEmail(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}

	logger.WithRequestID(logger.WithEmailHash(h.log, email), c.Request.Context()).
		WithField("action", "get_subscriptions").
		Info("user action")

	subscriptions, err := h.repo.GetSubscriptions(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}
