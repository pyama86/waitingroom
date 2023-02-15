package waitingroom

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetOutput(ioutil.Discard)
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
