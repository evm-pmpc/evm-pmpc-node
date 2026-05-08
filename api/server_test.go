package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(token string) *Server {
	return &Server{authToken: token}
}

func TestAuthenticate_NoTokenConfigured_AllowsAll(t *testing.T) {
	s := newTestServer("")
	called := false
	h := s.authenticate(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	h(rr, req)

	if !called {
		t.Fatal("handler not called when auth disabled")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthenticate_RejectsMissingHeader(t *testing.T) {
	s := newTestServer("secret")
	h := s.authenticate(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run on missing auth")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthenticate_RejectsTokenWithoutBearerPrefix(t *testing.T) {
	s := newTestServer("secret")
	h := s.authenticate(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run when 'Bearer ' prefix is missing")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "secret")
	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthenticate_RejectsWrongToken(t *testing.T) {
	s := newTestServer("secret")
	h := s.authenticate(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run on wrong token")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAuthenticate_AcceptsCorrectBearerToken(t *testing.T) {
	s := newTestServer("secret")
	called := false
	h := s.authenticate(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h(rr, req)

	if !called {
		t.Fatal("handler should have been called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthenticate_RejectsEmptyBearerValue(t *testing.T) {
	s := newTestServer("secret")
	h := s.authenticate(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run on empty bearer value")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// "Bearer " with nothing after.
	req.Header.Set("Authorization", strings.TrimRight("Bearer ", ""))
	h(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
