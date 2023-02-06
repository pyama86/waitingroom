package waitingroom

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/sirupsen/logrus"
)

const waitingInfoCookieKey = "waiting-room"
const allowedAccessCookieKey = "allowed-access"
const entryQueueCookieKey = "entry-queue"

func queueEntryKey(c echo.Context) string {
	return c.Param("domain") + "_queue_entry"
}

func hostSerialNumberKey(c echo.Context) string {
	return c.Param("domain") + "_serial_no"
}

func hostAllowedNumberKey(c echo.Context) string {
	return c.Param("domain") + "_allowed_no"
}

func enableDomainKey(c echo.Context) string {
	return c.Param("domain") + "_enable"
}

func enableDomainsKey() string {
	return "enable_domains"
}

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

// NewError エラー構造体を初期化して返却します
func NewError(statusCode int, err error, message string, params ...interface{}) *Error {
	if statusCode != http.StatusNotFound {
		logrus.Error(err, message)
	}
	m := fmt.Sprintf(message, params...)
	return &Error{StatusCode: statusCode, Message: m, RawErr: err}
}
