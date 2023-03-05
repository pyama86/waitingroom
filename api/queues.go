package api

import (
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/pyama86/ngx_waitingroom/model"
	"github.com/pyama86/ngx_waitingroom/waitingroom"
	validator "gopkg.in/go-playground/validator.v9"
)

type HTTPError struct {
	Code     int         `json:"-"`
	Message  interface{} `json:"message"`
	Internal error       `json:"-"` // Stores the error returned by an external dependency
}

// getQueues is getting queues.
// @Summary get queues
// @Description get queues
// @Accept  json
// @Produce  json
// @Param domain query string false "Queue"
// @Param limit query int false "Queue"
// @Param page query int false "page" minimum(1)
// @Param per_page query int false "per_page" minimum(1)
// @Success 200 {array} model.Queue
// @Failure 404 {array} model.Queue
// @Failure 500 {object} api.HTTPError
// @Router /queues [get]
// @Tags queues
func (h *queueHandler) getQueues(c echo.Context) error {
	page, err := strconv.ParseInt(c.QueryParam("page"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	if page == 0 {
		page = 1
	}
	perPage, err := strconv.ParseInt(c.QueryParam("per_page"), 10, 64)
	if perPage > 100 {
		perPage = 100
	}

	r, err := h.queueModel.GetQueues(c.Request().Context(), perPage, page)
	if err != nil {
		if err == redis.Nil {
			return c.JSON(http.StatusNotFound, err)
		}
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, r)
}

// updateQueueByName is update queue.
// @Summary update queue
// @Description update queue
// @Accept  json
// @Produce  json
// @Param domain path string true "Queue Name"
// @Param queue body model.Queue true "Queue Object"
// @Success 200 "OK"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /queues/{domain} [put]
// @Tags queues
func (h *queueHandler) updateQueueByName(c echo.Context) error {
	q := &model.Queue{}
	if err := c.Bind(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	if err := validator.New().Struct(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := h.queueModel.UpdateQueues(c.Request().Context(), c.Param("domain"), q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, nil)
}

// deleteQueueByName is delete queue.
// @Summary delete queue
// @Description delete queue
// @Accept  json
// @Produce  json
// @Param domain path string true "Queue Name"
// @Success 204 "No Content"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /queues/{domain} [delete]
// @Tags queues
func (h *queueHandler) deleteQueueByName(c echo.Context) error {
	if err := h.queueModel.DeleteQueues(c.Request().Context(), c.Param("domain")); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// createQueue is create queue.
// @Summary create queue
// @Description create queue
// @Accept  json
// @Produce  json
// @Param queue body model.Queue true "Queue Object"
// @Success 201 "Created"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /queues [post]
// @Tags queues
func (h *queueHandler) createQueue(c echo.Context) error {
	q := &model.Queue{}
	if err := c.Bind(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	if err := validator.New().Struct(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := h.queueModel.CreateQueues(c.Request().Context(), c.Param("domain"), q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, nil)

	return c.JSON(http.StatusCreated, nil)
}

type queueHandler struct {
	queueModel *model.QueueModel
}

func NewQueueHandler(redisC *redis.Client, config *waitingroom.Config, cache *waitingroom.Cache) *queueHandler {
	return &queueHandler{
		queueModel: model.NewQueueModel(redisC, config, cache),
	}
}
func QueuesEndpoints(g *echo.Group, redisC *redis.Client, config *waitingroom.Config, cache *waitingroom.Cache) {
	h := NewQueueHandler(redisC, config, cache)
	g.GET("/queues", h.getQueues)
	g.PUT("/queues/:domain", h.updateQueueByName)
	g.DELETE("/queues/:domain", h.deleteQueueByName)
	g.POST("/queues", h.createQueue)
}
