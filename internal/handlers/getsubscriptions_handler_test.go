package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"subber/internal/models"
)

func TestGetSubscriptions_InvalidInput(t *testing.T) {
	r := newTestRouter(&fakeHandlerRepo{}, &fakeSvc{})

	tests := []struct {
		name, email string
	}{
		{"missing email", ""},
		{"invalid email", "notanemail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/subscriptions/"
			if tt.email != "" {
				path += "?email=" + tt.email
			}
			w := do(r, http.MethodGet, path, nil)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestGetSubscriptions_DBError(t *testing.T) {
	r := newTestRouter(&fakeHandlerRepo{subsErr: errors.New("db error")}, &fakeSvc{})
	w := do(r, http.MethodGet, "/subscriptions/?email=a@b.com", nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestGetSubscriptions_ReturnsConfirmedSubscriptions(t *testing.T) {
	subs := []models.Subscription{{Email: "a@b.com", Repo: "owner/repo", Confirmed: true}}
	r := newTestRouter(&fakeHandlerRepo{subs: subs}, &fakeSvc{})

	w := do(r, http.MethodGet, "/subscriptions/?email=a@b.com", nil)

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
