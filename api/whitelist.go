package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/pyama86/ngx_waitingroom/model"
	validator "gopkg.in/go-playground/validator.v9"
)

func paginate(c echo.Context) (int64, int64, error) {
	page := int64(0)
	perPage := int64(100)
	if c.QueryParam("page") != "" {
		pa, err := strconv.ParseInt(c.QueryParam("page"), 10, 64)
		if err != nil {
			return 0, 0, err
		}
		page = pa
	}

	if page == 0 {
		page = 1
	}

	if c.QueryParam("per_page") != "" {
		perP, err := strconv.ParseInt(c.QueryParam("per_page"), 10, 64)
		if err != nil {
			return 0, 0, err
		}

		if perP > 100 {
			perP = 100
		}
		perPage = perP
	}

	c.Response().Header().Set("X-Pagination-Current-Page", strconv.FormatInt(page, 10))
	c.Response().Header().Set("X-Pagination-Limit", strconv.FormatInt(perPage, 10))
	return page, perPage, nil
}

// getWhiteList is getting whiteLists.
// @Summary get whiteLists
// @Description get whiteLists
// @ID whitelist#get
// @Accept  json
// @Produce  json
// @Param domain query string false "WhiteList Domain"
// @Param page query int false "page" minimum(1)
// @Param per_page query int false "per_page" minimum(1)
// @Success 200 {array} model.WhiteList
// @Failure 404 {array} model.WhiteList
// @Failure 500 {object} api.HTTPError
// @Router /whitelist [get]
// @Tags whitelist
func (h *whiteListHandler) getWhiteList(c echo.Context) error {
	page, perPage, err := paginate(c)
	if err != nil {
		slog.Error("pagenate error", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, err)
	}

	r, total, err := h.whiteListModel.GetWhiteList(c.Request().Context(), perPage, page)
	if err != nil {
		if err == redis.Nil {
			return c.JSON(http.StatusNotFound, err)
		}
		slog.Error("cant get whiteList", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, err)
	}
	c.Response().Header().Set("X-Pagination-Total-Pages", strconv.FormatInt(total, 10))
	return c.JSON(http.StatusOK, r)
}

// deleteWhiteListByName is delete whiteList.
// @Summary delete whiteList
// @Description delete whiteList
// @ID whitelist#delete
// @Accept  json
// @Produce  json
// @Param domain path string true "WhiteList Domain"
// @Success 204 "No Content"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /whitelist/{domain} [delete]
// @Tags whitelist
func (h *whiteListHandler) deleteWhiteListByName(c echo.Context) error {
	if err := h.whiteListModel.DeleteWhiteList(c.Request().Context(), c.Param("domain")); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// createWhiteList is create whiteList.
// @Summary create whiteList
// @Description create whiteList
// @ID whitelist#post
// @Accept  json
// @Produce  json
// @Param whitelist body model.WhiteList true "WhiteList Object"
// @Success 201 "Created"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /whitelist [post]
// @Tags whitelist
func (h *whiteListHandler) createWhiteList(c echo.Context) error {
	q := &model.WhiteList{}
	if err := c.Bind(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	if err := validator.New().Struct(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := h.whiteListModel.CreateWhiteList(c.Request().Context(), q.Domain); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusCreated, nil)
}

type whiteListHandler struct {
	whiteListModel *model.WhiteListModel
}

func NewWhiteListHandler(redisC *redis.Client) *whiteListHandler {
	return &whiteListHandler{
		whiteListModel: model.NewWhiteListModel(redisC),
	}
}

func WhiteListEndpoints(g *echo.Group, redisC *redis.Client) {
	h := NewWhiteListHandler(redisC)
	g.GET("/whitelist", h.getWhiteList)
	g.DELETE("/whitelist/:domain", h.deleteWhiteListByName)
	g.POST("/whitelist", h.createWhiteList)
}
