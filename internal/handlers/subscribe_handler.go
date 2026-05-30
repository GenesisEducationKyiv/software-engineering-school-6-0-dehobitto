package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"subber/internal/models"
	"subber/internal/service"
	"subber/internal/validators"
)

func (h *Handler) Subscribe(c *gin.Context) {
	var input models.Subscription

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if !validators.IsValidRepo(input.Repo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository format"})
		return
	}

	handlerLog.WithField("action", "subscribe").WithField("email", input.Email).WithField("repo", input.Repo).Info("user action")

	err := h.svc.Subscribe(c.Request.Context(), input.Email, input.Repo)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{"success": "Subscription successful. Confirmation email sent."})
		return
	}

	switch {
	case errors.Is(err, service.ErrAlreadySubscribed):
		c.JSON(http.StatusConflict, gin.H{"error": "Email already subscribed to this repository"})
	case errors.Is(err, service.ErrRepoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found on GitHub"})
	case errors.Is(err, service.ErrGitHubRateLimit):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "GitHub API rate limit exceeded. Try again later."})
	case errors.Is(err, service.ErrGitHubUnavailable):
		c.JSON(http.StatusBadGateway, gin.H{"error": "External API error"})
	default:
		handlerLog.WithField("email", input.Email).WithField("repo", input.Repo).WithError(err).Error("subscribe failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}
