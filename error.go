package main

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

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
		logrus.Error(err)
	}
	m := fmt.Sprintf(message, params...)
	return &Error{StatusCode: statusCode, Message: m, RawErr: err}
}
