package waitingroom

import (
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

type QueueBase struct {
	sc     *securecookie.SecureCookie
	config *Config
}

func (q *QueueBase) setWaitingInfoCookie(c echo.Context, waitingInfo *WaitingInfo) error {
	encoded, err := q.sc.Encode(waitingInfoCookieKey, waitingInfo)
	if err != nil {
		return err
	}

	c.SetCookie(&http.Cookie{
		Name:     waitingInfoCookieKey,
		Value:    encoded,
		MaxAge:   q.config.ClientPollingIntervalSec * 2,
		Domain:   c.Param("domain"),
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
	return nil
}
