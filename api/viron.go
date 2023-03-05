package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// vironAuthType
// @Summary get auth type
// @Description get auth type
// @ID viron_authtype#get
// @Accept  json
// @Produce  json
// @Router /viron_authtype [get]
// @Tags viron
func vironAuthType(c echo.Context) error {
	encodedJSON := []byte(`[]`)
	return c.JSONBlob(http.StatusOK, encodedJSON)

}

//vironGlobalMenu
// @Summary get global menu
// @Description get global menu
// @ID viron#get
// @Accept json
// @Produce json
// @Router /viron [get]
// @Tags viron
func vironGlobalMenu(c echo.Context) error {
	encodedJSON := []byte(`{
  "theme": "standard",
  "color": "white",
  "name": "WaitingRoom",
  "tags": [
    "queues",
    "whitelist"
  ],
  "pages": [
    {
      "section": "manage",
      "id": "queues",
      "name": "Queues",
      "components": [
        {
          "api": {
            "method": "get",
            "path": "/queues"
          },
	  "query": [
	    { key: "domain", type: "string" },
          ],
	  "primary": "domain",
          "name": "Queue",
	  "style": "table",
          "pagination": true,
	  "table_labels": [
	    "domain",
            "current_no",
            "permitted_no",
	  ]
        }
      ]
    },
    {
      "section": "manage",
      "id": "whitelist",
      "name": "WhiteList",
      "components": [
        {
          "api": {
            "method": "get",
            "path": "/whitelist"
          },
	  "query": [
	    { key: "domain", type: "string" },
          ],
          "name": "WhiteList",
	  "style": "table",
	  "primary": "name",
	  "table_labels": [
	    "domain",
	  ]
        }
      ]
    }
  ]
}`)
	return c.JSONBlob(http.StatusOK, encodedJSON)

}

func VironEndpoints(g *echo.Group) {
	g.GET("/viron", vironGlobalMenu)
	g.GET("/viron_authtype", vironAuthType)
}
