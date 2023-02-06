package waitingroom

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

func postContext(path string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	values := make(url.Values)
	for k, v := range params {
		values.Add(k, v)
	}

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))

	e := echo.New()
	ctx := e.NewContext(req, rec)
	return ctx, rec
}
