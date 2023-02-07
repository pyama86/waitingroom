package waitingroom

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

// NewError エラー構造体を初期化して返却します
func NewError(statusCode int, err error, message string, params ...interface{}) *Error {
	if statusCode != http.StatusNotFound {
		logrus.Error(err, message)
	}
	m := fmt.Sprintf(message, params...)
	return &Error{StatusCode: statusCode, Message: m, RawErr: err}
}
