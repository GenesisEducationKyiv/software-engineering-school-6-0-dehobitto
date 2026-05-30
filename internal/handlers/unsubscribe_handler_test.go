package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestUnsubscribeByToken(t *testing.T) {
	validToken := uuid.New().String()

	tests := []struct {
		name     string
		token    string
		unsubErr error
		want     int
	}{
		// 400: malformed token — caught before any DB call
		{"invalid uuid", "not-valid", nil, http.StatusBadRequest},
		// 404: token not in DB — already unsubscribed or link is stale
		{"unknown token", validToken, errors.New("not found"), http.StatusNotFound},
		// 200: unsubscribed successfully
		{"success", validToken, nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&fakeHandlerRepo{unsubErr: tt.unsubErr}, &fakeSvc{})
			w := do(r, http.MethodGet, "/unsubscribe/"+tt.token, nil)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}
