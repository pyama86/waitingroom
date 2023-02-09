package waitingroom

import (
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

const waitingInfoCookieKey = "waiting-room"
const paramDomainKey = "domain"
const enableDomainKey = "queue-domains"

// Error Apiのエラーを定義する構造体
type Error struct {
	StatusCode int
	Message    string
	RawErr     error
}

// Error Errorインターフェースの必須定義メソッド
func (err *Error) Error() string {
	if err.RawErr != nil {
		return err.RawErr.Error()
	}
	return err.Message
}

func (err *Error) Unwrap() error {
	return err.RawErr
}

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
		Domain:   c.Param(paramDomainKey),
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
	return nil
}

func (q *QueueBase) hostCurrentNumberKey(c echo.Context) string {
	return c.Param(paramDomainKey) + "_current_no"
}

func (q *QueueBase) allowNoKey(c echo.Context) string {
	return c.Param(paramDomainKey) + "_allow_no"
}

func (q *QueueBase) lockAllowNoKey(domain string) string {
	return domain + "_lock_allow_no"
}
