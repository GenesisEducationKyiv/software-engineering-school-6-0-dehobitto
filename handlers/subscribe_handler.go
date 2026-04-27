package handlers

import (
	"fmt"
	"log"
	"net/http"

	"subber/models"
	"subber/utils"
	"subber/utils/github"
	"subber/workers"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) Subscribe(c *gin.Context) {
	var newOwnerRepo models.Subscription

	if err := c.ShouldBindJSON(&newOwnerRepo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if !utils.IsValidRepo(newOwnerRepo.Repo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository format"})
		return
	}

	exists, err := h.repo.SubscriptionExists(c.Request.Context(), newOwnerRepo.Email, newOwnerRepo.Repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error during check"})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already subscribed to this repository"})
		return
	}

	resp, err := github.CheckIfRepoExists(c.Request.Context(), newOwnerRepo.Repo, h.cfg.GitHubToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reach GitHub API"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "GitHub API rate limit exceeded. Try again later."})
		return
	}

	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found on GitHub"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "External API error"})
		return
	}

	tag, err := github.GetLatestTag(c.Request.Context(), newOwnerRepo.Repo, h.cfg.GitHubToken, h.cache)
	if err != nil {
		log.Printf("Warning: Could not fetch initial tag for %s: %v", newOwnerRepo.Repo, err)
	}

	newOwnerRepo.LastSeenTag = tag
	newOwnerRepo.Token = uuid.New().String()
	newOwnerRepo.Confirmed = false

	err = h.repo.SaveSubscription(c.Request.Context(), newOwnerRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database saving failed"})
		return
	}

	h.sendConfirmation(newOwnerRepo.Email, newOwnerRepo.Token)

	c.JSON(http.StatusOK, gin.H{"success": "Subscription successful. Confirmation email sent."})
}

func (h *Handler) sendConfirmation(email, token string) {
	confirmURL := fmt.Sprintf("http://localhost:%s/api/confirm/%s", h.cfg.ServerPort, token)

	message := fmt.Sprintf(
		"Welcome! Please confirm your subscription to GitHub repository updates by clicking here: %s",
		confirmURL,
	)

	job := workers.NotificationJob{
		Email:   email,
		Message: message,
	}

	select {
	case h.jobs <- job:
		log.Printf("Confirmation job queued for: %s", email)
	default:
		log.Printf("Critical: Notification channel is full. Dropping confirmation for: %s", email)
	}
}
