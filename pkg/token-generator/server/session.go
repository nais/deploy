package server

import (
	"encoding/gob"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

var (
	// Keep sessions in-memory, and invalidate them on every program launch.
	sessionStore = sessions.NewCookieStore(securecookie.GenerateRandomKey(32))
)

const (
	sessionIdentifier = `token-generator`
)

type Session struct {
	Token *oauth2.Token
	State string
}

func init() {
	gob.Register(&Session{})
}

func GetSession(r *http.Request) (*Session, func(w http.ResponseWriter, r *http.Request) error, error) {
	session, err := sessionStore.Get(r, sessionIdentifier)
	if err != nil {
		// in case there is an error with our current session data, just reset it.
		if session == nil {
			panic(err)
		}
	}

	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
	}

	var data = &Session{}
	var ok bool

	if data, ok = session.Values["data"].(*Session); !ok {
		// session cookie has invalid data.
		data = &Session{}
	}

	save := func(w http.ResponseWriter, r *http.Request) error {
		session.Values["data"] = data
		return session.Save(r, w)
	}

	return data, save, nil
}
