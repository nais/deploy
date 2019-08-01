package server

import (
	"net/http"
	"sync"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

var (
	// Keep sessions in-memory, and invalidate them on every program launch.
	sessions    = make(map[string]Session)
	sessionLock sync.Mutex
)

const (
	sessionCookie = "X-Token-Generator-Session"
)

type Session struct {
	id    string
	Token *oauth2.Token
	JWT   *jwt.Token
	State string
}

func (s *Session) ID() string {
	sessionLock.Lock()
	defer sessionLock.Unlock()
	if len(s.id) == 0 {
		s.id = uuid.New().String()
	}
	return s.id
}

func (s *Session) Cookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    s.ID(),
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
	}
}

func (s *Session) Save() {
	sessionLock.Lock()
	defer sessionLock.Unlock()
	sessions[s.ID()] = *s
}

func GetSession(r *http.Request) Session {
	cookie, err := r.Cookie(sessionCookie)
	if cookie == nil || err != nil {
		return Session{}
	}

	return sessions[cookie.Value]
}
