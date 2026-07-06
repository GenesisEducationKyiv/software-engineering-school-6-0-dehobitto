package delivery

import (
	"encoding/json"
	"net/http"

	"subber/pkg/logger"
)

type ConfirmationRequest struct {
	Email      string `json:"email"`
	Repo       string `json:"repo"`
	ConfirmURL string `json:"confirm_url"`
}

func NewHTTPHandler(service *Service, log logger.Logger) http.Handler {
	if log == nil {
		log = logger.NewNoop()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/notifications/confirmation", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request ConfirmationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if request.Email == "" || request.Repo == "" || request.ConfirmURL == "" {
			http.Error(w, "email, repo and confirm_url are required", http.StatusBadRequest)
			return
		}
		if err := service.SendConfirmation(r.Context(), request.Email, request.Repo, request.ConfirmURL); err != nil {
			log.WithField("repo", request.Repo).WithError(err).Error("confirmation notification failed")
			http.Error(w, "notification failed", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	return mux
}
