package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
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
// @ID queues#get
// @Accept  json
// @Produce  json
// @Param domain query string false "Queue Domain"
// @Param page query int false "page" minimum(1)
// @Param per_page query int false "per_page" minimum(1)
// @Success 200 {array} model.Queue
// @Failure 404 {array} model.Queue
// @Failure 500 {object} api.HTTPError
// @Router /queues [get]
// @Tags queues
func (h *queueHandler) GetQueues(c echo.Context) error {
	page, perPage, err := paginate(c)
	if err != nil {
		slog.Error("pagenate error", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, err)
	}

	r, total, err := h.queueModel.GetQueues(c.Request().Context(), perPage, page)
	if err != nil {
		if err == redis.Nil {
			return c.JSON(http.StatusNotFound, err)
		}
		slog.Error("can't get queues", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, err)
	}
	c.Response().Header().Set("X-Pagination-Total-Pages", strconv.FormatInt(total, 10))
	return c.JSON(http.StatusOK, r)
}

// updateQueueByName is update queue.
// @Summary update queue
// @Description update queue
// @ID queues#put
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
func (h *queueHandler) UpdateQueueByName(c echo.Context) error {
	q := &model.Queue{}
	if err := c.Bind(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	if err := validator.New().Struct(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := h.queueModel.UpdateQueues(c.Request().Context(), q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusOK, nil)
}

// deleteQueueByName is delete queue.
// @Summary delete queue
// @Description delete queue
// @ID queues#delete
// @Accept  json
// @Produce  json
// @Param domain path string true "Queue Name"
// @Success 204 "No Content"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /queues/{domain} [delete]
// @Tags queues
func (h *queueHandler) DeleteQueueByName(c echo.Context) error {
	if err := h.queueModel.DeleteQueues(c.Request().Context(), c.Param("domain")); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	return c.NoContent(http.StatusNoContent)
}

// createQueue is create queue.
// @Summary create queue
// @Description create queue
// @ID queues#post
// @Accept  json
// @Produce  json
// @Param queue body model.Queue true "Queue Object"
// @Success 201 "Created"
// @Failure 403 {object} api.HTTPError
// @Failure 404 {object} api.HTTPError
// @Failure 500 {object} api.HTTPError
// @Router /queues [post]
// @Tags queues
func (h *queueHandler) CreateQueue(c echo.Context) error {
	q := &model.Queue{}
	if err := c.Bind(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	if err := validator.New().Struct(q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}

	if err := h.queueModel.CreateQueues(c.Request().Context(), q); err != nil {
		return c.JSON(http.StatusBadRequest, err)
	}
	return c.JSON(http.StatusCreated, nil)
}

type queueHandler struct {
	queueModel  *model.QueueModel
	sc          *securecookie.SecureCookie
	cache       *waitingroom.Cache
	redisClient *redis.Client
	config      *waitingroom.Config
}

func NewQueueHandler(
	sc *securecookie.SecureCookie,
	redisC *redis.Client,
	config *waitingroom.Config,
	cache *waitingroom.Cache,
) *queueHandler {
	return &queueHandler{
		sc:          sc,
		queueModel:  model.NewQueueModel(redisC, config, cache),
		redisClient: redisC,
		config:      config,
		cache:       cache,
	}
}

const paramDomainKey = "domain"

type QueueResult struct {
	ID                  string
	Enabled             bool  `json:"enabled"`
	PermittedClient     bool  `json:"permitted_client"`
	SerialNo            int64 `json:"serial_no"`
	PermittedNo         int64 `json:"permitted_no"`
	RemainingWaitSecond int64 `json:"remaining_wait_second"`
}

func (p *queueHandler) Check(c echo.Context) error {

	site := waitingroom.NewSite(c.Request().Context(), c.Param(paramDomainKey), p.config, p.redisClient, p.cache)

	// 歴史的な経緯でGETでwaitingroomを有効にしているが、POSTで有効にするべき
	if c.Param("enable") != "" {
		if err := site.EnableQueue(); err != nil {
			return newError(http.StatusInternalServerError, err, " can't enable queue")
		}
	} else {
		ok, err := site.IsEnabledQueue(true)
		if err != nil {
			return newError(http.StatusInternalServerError, err, " can't get enable status")
		}

		if !ok {
			return c.JSON(http.StatusOK, QueueResult{Enabled: false, PermittedClient: false})
		}

	}

	// ホワイトリストに含まれているドメインならば即時許可応答する
	ok, err := site.IsInWhitelist()
	if err != nil {
		return newError(http.StatusInternalServerError, err, " can't get whitelist")
	}
	if ok {
		return c.JSON(http.StatusOK, QueueResult{Enabled: false, PermittedClient: false})
	}

	// 許可済みクライアントかどうかを判定する
	client, err := waitingroom.NewClientByContext(c, p.sc)
	if err != nil {
		return newError(http.StatusInternalServerError, err, " can't build info")
	}
	ok, err = site.IsPermittedClient(client)
	if err != nil {
		return newError(http.StatusInternalServerError, err, " can't get permit status")
	}

	if ok {
		return c.JSON(http.StatusOK, QueueResult{ID: client.ID, Enabled: true, PermittedClient: true})
	}

	clientSerialNumber, err := site.AssignSerialNumber(client)
	if err != nil {
		return newError(http.StatusInternalServerError, err, " can't get serial no")
	}

	if err := client.SaveToCookie(c, p.config); err != nil {
		return newError(http.StatusInternalServerError, err, "can't save client info")
	}

	if clientSerialNumber != 0 {
		ok, err := site.CheckAndPermitClient(client)
		if err != nil {
			return newError(http.StatusInternalServerError, err, " can't jude permit access")
		}
		if ok {
			return c.JSON(http.StatusOK, QueueResult{ID: client.ID, Enabled: true, PermittedClient: true})
		}
	}

	remaningWaitSecond, pn, err := site.CalcRemainingWaitSecond(client)
	if err != nil {
		return newError(http.StatusInternalServerError, err, " can't calc remaining wait second")
	}
	return c.JSON(http.StatusTooManyRequests, QueueResult{
		ID:                  client.ID,
		Enabled:             true,
		PermittedClient:     false,
		SerialNo:            client.SerialNumber,
		PermittedNo:         pn,
		RemainingWaitSecond: remaningWaitSecond,
	})
}
