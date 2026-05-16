package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfirmByToken_InvalidToken(t *testing.T) {
	r := newTokenRouter(&Handler{})

	req := httptest.NewRequest("GET", "/api/confirm/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
