package api

import (
	"fmt"
	"log/slog"
	"net/http"
)

// newError エラー構造体を初期化して返却します
func newError(statusCode int, err error, message string, params ...interface{}) *Error {
	if statusCode != http.StatusNotFound {
		slog.Error(
			"error",
			slog.String("message", fmt.Sprintf(message, params...)),
			slog.String("error", err.Error()),
		)
	}
	m := fmt.Sprintf(message, params...)
	return &Error{StatusCode: statusCode, Message: m, RawErr: err}
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
