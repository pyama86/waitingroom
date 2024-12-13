package waitingroom

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
)

func init() {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	slog.SetDefault(logger)

}
func testContext(path, method string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	postBody, _ := json.Marshal(params)

	req := httptest.NewRequest(method, path, bytes.NewBuffer(postBody))
	req.Header.Set("Content-Type", "application/json")
	e := echo.New()
	ctx := e.NewContext(req, rec)
	return ctx, rec
}
