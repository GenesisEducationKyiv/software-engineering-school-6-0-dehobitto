package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"

	"subber/internal/models"
)

func TestGetSubscriptions_InvalidInput(t *testing.T) {
	tests := []struct {
		name, email string
	}{
		{"missing email", ""},
		{"invalid email", "notanemail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockSubscriptionRepository)
			r := newTestRouter(repo, new(mockSubscriptionService))

			path := "/subscriptions/"
			if tt.email != "" {
				path += "?email=" + tt.email
			}
			w := do(r, http.MethodGet, path, nil)
			repo.AssertNotCalled(t, "GetSubscriptions", mock.Anything, mock.Anything)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestGetSubscriptions_DBError(t *testing.T) {
	repo := new(mockSubscriptionRepository)
	repo.On("GetSubscriptions", mock.Anything, "a@b.com").
		Return([]models.Subscription(nil), errors.New("db error")).
		Once()
	r := newTestRouter(repo, new(mockSubscriptionService))

	w := do(r, http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	repo.AssertExpectations(t)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGetSubscriptions_ReturnsConfirmedSubscriptions(t *testing.T) {
	subs := []models.Subscription{{Email: "a@b.com", Repo: "owner/repo", Confirmed: true}}
	repo := new(mockSubscriptionRepository)
	repo.On("GetSubscriptions", mock.Anything, "a@b.com").Return(subs, nil).Once()
	r := newTestRouter(repo, new(mockSubscriptionService))

	w := do(r, http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	repo.AssertExpectations(t)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got []models.Subscription
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Repo != "owner/repo" {
		t.Errorf("body = %v, want one subscription for owner/repo", got)
	}
}
