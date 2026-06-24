package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"subber/pkg/logger"
	"subber/pkg/requestid"
	"subber/services/subscription-api/internal/subscription"
)

var (
	repoPattern  = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
	emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

type RouterDeps struct {
	APIKey            string
	Repo              SubscriptionReader
	Service           SubscriptionCreator
	Logger            logger.Logger
	Gatherer          prometheus.Gatherer
	AccessMetrics     *AccessMetrics
	SubscribeRequests *prometheus.CounterVec
}

type SubscriptionReader interface {
	GetSubscriptions(ctx context.Context, email string) ([]subscription.Subscription, error)
	ConfirmSubscriptionByToken(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
}

type SubscriptionCreator interface {
	Subscribe(ctx context.Context, email, repo string) error
}

func SetupRouter(deps RouterDeps) *gin.Engine {
	log := deps.Logger
	if log == nil {
		log = logger.NewNoop()
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.SetTrustedProxies(nil) //nolint:errcheck,gosec
	r.Use(requestIDMiddleware())
	r.Use(accessMiddleware(log.WithField("component", "access"), deps.AccessMetrics))

	if deps.Gatherer != nil {
		r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(deps.Gatherer, promhttp.HandlerOpts{})))
	}
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	h := handler{
		repo:              deps.Repo,
		service:           deps.Service,
		log:               log.WithField("component", "handler"),
		subscribeRequests: deps.SubscribeRequests,
	}
	api := r.Group("/api")
	api.GET("/confirm/:token", h.confirm)
	api.GET("/unsubscribe/:token", h.unsubscribe)

	protected := api.Group("/")
	protected.Use(apiKeyAuth(deps.APIKey))
	protected.POST("/subscribe", h.subscribe)
	protected.GET("/subscriptions/", h.getSubscriptions)

	return r
}

type handler struct {
	repo              SubscriptionReader
	service           SubscriptionCreator
	log               logger.Logger
	subscribeRequests *prometheus.CounterVec
}

func (h handler) subscribe(c *gin.Context) {
	var input subscription.Subscription
	if err := c.ShouldBindJSON(&input); err != nil {
		h.observeSubscribe("invalid_json")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	if !emailPattern.MatchString(input.Email) {
		h.observeSubscribe("invalid_email")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}
	if !repoPattern.MatchString(input.Repo) {
		h.observeSubscribe("invalid_repo")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository format"})
		return
	}

	err := h.service.Subscribe(c.Request.Context(), input.Email, input.Repo)
	if err == nil {
		h.observeSubscribe("success")
		c.JSON(http.StatusOK, gin.H{"success": "Subscription successful. Confirmation email sent."})
		return
	}
	switch {
	case errors.Is(err, subscription.ErrAlreadySubscribed):
		h.observeSubscribe("already_subscribed")
		c.JSON(http.StatusConflict, gin.H{"error": "Email already subscribed to this repository"})
	case errors.Is(err, subscription.ErrRepoNotFound):
		h.observeSubscribe("repo_not_found")
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found on GitHub"})
	case errors.Is(err, subscription.ErrGitHubRateLimit):
		h.observeSubscribe("github_rate_limit")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "GitHub API rate limit exceeded. Try again later."})
	case errors.Is(err, subscription.ErrGitHubUnavailable):
		h.observeSubscribe("github_unavailable")
		c.JSON(http.StatusBadGateway, gin.H{"error": "External API error"})
	default:
		h.observeSubscribe("error")
		h.log.WithField("repo", input.Repo).WithError(err).Error("subscribe failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
	}
}

func (h handler) observeSubscribe(result string) {
	if h.subscribeRequests != nil {
		h.subscribeRequests.WithLabelValues(result).Inc()
	}
}

func (h handler) getSubscriptions(c *gin.Context) {
	email := c.Query("email")
	if email == "" || !emailPattern.MatchString(email) {
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

func (h handler) confirm(c *gin.Context) {
	token := c.Param("token")
	if err := uuid.Validate(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
		return
	}
	if err := h.repo.ConfirmSubscriptionByToken(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Subscription confirmed successfully"})
}

func (h handler) unsubscribe(c *gin.Context) {
	token := c.Param("token")
	if err := uuid.Validate(token); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
		return
	}
	if err := h.repo.Unsubscribe(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed successfully"})
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := requestid.Normalize(c.GetHeader(requestid.Header))
		c.Writer.Header().Set(requestid.Header, id)
		c.Request = c.Request.WithContext(requestid.WithContext(c.Request.Context(), id))
		c.Next()
	}
}

func apiKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-API-Key") == apiKey {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	}
}
