package session

// Package session provides map-based in-memory HTTP client session based on cookies.

import (
	"net/http"
	"sync"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	// Keep sessions in-memory, and invalidate them on every program launch.
	sessions = make(map[string]Session)

	// Thread safe.
	sessionLock sync.Mutex
)

const (
	sessionCookie = "X-Token-Generator-Session"
)

// Contents of the JWT
type Claims struct {
	jwt.MapClaims
	UPN string `json:"upn"`
}

// Contents of the session object.
type Session struct {
	id     string
	Token  *oauth2.Token
	JWT    *jwt.Token
	Claims Claims
	State  string
}

// Generate a unique session ID if not already exists,
// then return it.
func (s *Session) ID() string {
	if len(s.id) == 0 {
		sessionLock.Lock()
		s.id = uuid.New().String()
		sessionLock.Unlock()
	}
	return s.id
}

// Generate a HTTP cookie with the session ID.
func (s *Session) Cookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    s.ID(),
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
	}
}

// Persist the session to memory.
func (s *Session) Save() {
	id := s.ID()
	sessionLock.Lock()
	sessions[id] = *s
	sessionLock.Unlock()
}

// Extract a session object from a HTTP request.
// If no cookie exists already, a new one will be returned.
func GetSession(r *http.Request) Session {
	cookie, err := r.Cookie(sessionCookie)
	if cookie == nil || err != nil {
		return Session{}
	}

	return sessions[cookie.Value]
}
