package waitingroom

import (
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetOutput(ioutil.Discard)
}
func testContext(path, method string, params map[string]string) (echo.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	values := make(url.Values)
	for k, v := range params {
		values.Add(k, v)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e := echo.New()
	ctx := e.NewContext(req, rec)
	return ctx, rec
}
