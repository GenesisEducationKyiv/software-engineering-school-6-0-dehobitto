package handlers

import (
	"net/http"
	"subber/utils"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetSubscriptions(c *gin.Context) {
	email := c.Query("email")

	if email == "" || !utils.IsValidEmail(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}

	subscriptions, err := h.repo.GetSubscriptions(c.Request.Context(), email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, subscriptions)
}
