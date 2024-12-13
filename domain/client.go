package waitingroom

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

const paramDomainKey = "domain"

type Client struct {
	SerialNumber         int64  // 通し番号
	ID                   string // ユーザー固有ID
	TakeSerialNumberTime int64  // シリアルナンバーを取得するUNIXTIME
	secureCookie         *securecookie.SecureCookie
	domain               string
}

const ClientCookieKey = "waiting-room"

func NewClientByContext(ctx echo.Context, sc *securecookie.SecureCookie) (*Client, error) {
	cookie, err := ctx.Cookie(ClientCookieKey)
	if err != nil {
		if err != http.ErrNoCookie {
			return nil, err
		}
	}

	client := Client{}
	if cookie != nil {
		if err = sc.Decode(ClientCookieKey,
			cookie.Value,
			&client); err != nil {
			ctx.SetCookie(&http.Cookie{
				Name:     ClientCookieKey,
				MaxAge:   -1,
				Domain:   ctx.Param(paramDomainKey),
				Path:     "/",
				Secure:   true,
				HttpOnly: true,
			})
			return nil, fmt.Errorf("can't decode cookie :%s", err)
		}
	}
	client.secureCookie = sc
	client.domain = ctx.Param(paramDomainKey)

	return &client, nil
}

func (c *Client) canTakeSerialNumber() bool {
	return c.ID != "" && c.SerialNumber == 0 && c.TakeSerialNumberTime > 0 && c.TakeSerialNumberTime < time.Now().Unix()
}

func (c *Client) SaveToCookie(ctx echo.Context, config *Config) error {
	encoded, err := c.secureCookie.Encode(ClientCookieKey, c)
	if err != nil {
		return err
	}

	ctx.SetCookie(&http.Cookie{
		Name:     ClientCookieKey,
		Value:    encoded,
		MaxAge:   config.PermittedAccessSec,
		Domain:   c.domain,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
	return nil
}
func (c *Client) AssignID(delaySec int64) error {
	u, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	c.ID = u.String()
	c.TakeSerialNumberTime = time.Now().Unix() + delaySec
	c.SerialNumber = 0
	return nil
}
func (c *Client) AssignSerialNumber(sn int64) {
	c.SerialNumber = sn
}

func (c *Client) HasID() bool {
	return c.ID != ""
}

func (c *Client) IsPermitClient(an int64) bool {
	return c.SerialNumber != 0 && an >= c.SerialNumber
}

func (c *Client) HasSerialNumber() bool {
	return c.SerialNumber != 0 && c.ID != ""
}
