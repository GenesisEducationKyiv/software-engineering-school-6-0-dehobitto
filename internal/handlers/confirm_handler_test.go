package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestConfirmByToken(t *testing.T) {
	validToken := uuid.New().String()

	tests := []struct {
		name       string
		token      string
		confirmErr error
		want       int
	}{
		// 400: malformed token — caught before any DB call
		{"invalid uuid", "not-a-uuid", nil, http.StatusBadRequest},
		// 404: token not in DB — link is stale or already used
		{"unknown token", validToken, errors.New("not found"), http.StatusNotFound},
		// 200: subscription confirmed
		{"success", validToken, nil, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&fakeHandlerRepo{confirmErr: tt.confirmErr}, &fakeSvc{})
			w := do(r, http.MethodGet, "/confirm/"+tt.token, nil)
			if w.Code != tt.want {
				t.Errorf("status = %d, want %d", w.Code, tt.want)
			}
		})
	}
}
