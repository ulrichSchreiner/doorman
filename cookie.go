package doorman

import (
	"fmt"
	"net/http"

	"github.com/gorilla/securecookie"
	"go.uber.org/zap"
)

const (
	cookieName = "doorman"
)

func newRandomKey(n int) []byte {
	return securecookie.GenerateRandomKey(n)
}

type cookieData map[string]interface{}

type cookieHandler struct {
	*zap.Logger
	insecure bool
	domain   string
	c        *securecookie.SecureCookie
}

func newCookie(logger *zap.Logger, hash, block []byte, insecure bool, domain string) *cookieHandler {
	return &cookieHandler{
		Logger:   logger,
		insecure: insecure,
		domain:   domain,
		c:        securecookie.New(hash, block)}
}

func (ch *cookieHandler) set(w http.ResponseWriter, value cookieData) {
	if encoded, err := ch.c.Encode(cookieName, value); err == nil {
		cookie := &http.Cookie{
			Name:     cookieName,
			Domain:   ch.domain,
			Value:    encoded,
			Path:     "/",
			Secure:   !ch.insecure,
			HttpOnly: true,
		}
		http.SetCookie(w, cookie)
	} else {
		ch.Error("cannot encode cookie", zap.Error(err))
	}
}

func (ch *cookieHandler) get(r *http.Request) (cookieData, error) {
	if cookie, err := r.Cookie(cookieName); err == nil {
		values := make(cookieData)
		if err = ch.c.Decode(cookieName, cookie.Value, &values); err == nil {
			return values, nil
		}
		return nil, fmt.Errorf("cookie cannot be decoded")
	}
	return nil, fmt.Errorf("no cookie found")
}
