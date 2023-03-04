package waitingroom

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
)

type Client struct {
	SerialNumber         int64  // 通し番号
	ID                   string // ユーザー固有ID
	TakeSerialNumberTime int64  // シリアルナンバーを取得するUNIXTIME
	secureCookie         *securecookie.SecureCookie
	domain               string
}

const clientCookieKey = "waiting-room"

func NewClientByContext(ctx echo.Context, sc *securecookie.SecureCookie) (*Client, error) {
	cookie, err := ctx.Cookie(clientCookieKey)
	if err != nil {
		if err != http.ErrNoCookie {
			return nil, err
		}
	}

	client := Client{}
	if cookie != nil {
		if err = sc.Decode(clientCookieKey,
			cookie.Value,
			&client); err != nil {
			ctx.SetCookie(&http.Cookie{
				Name:     clientCookieKey,
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

func (c *Client) fillSerialNumber(site *Site) (int64, error) {
	if c.SerialNumber != 0 && c.ID != "" {
		return c.SerialNumber, nil
	}

	if c.ID == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return 0, err
		}
		c.ID = u.String()
		c.TakeSerialNumberTime = time.Now().Unix() + site.config.EntryDelaySec
		c.SerialNumber = 0
	} else if c.canTakeSerialNumber() {
		currentNo, err := site.IncrCurrentNumber()
		if err != nil {
			return 0, err
		}
		c.SerialNumber = currentNo
	}
	return c.SerialNumber, nil
}

func (c *Client) saveToCookie(ctx echo.Context, config *Config) error {
	encoded, err := c.secureCookie.Encode(clientCookieKey, c)
	if err != nil {
		return err
	}

	ctx.SetCookie(&http.Cookie{
		Name:     clientCookieKey,
		Value:    encoded,
		MaxAge:   config.PermittedAccessSec,
		Domain:   c.domain,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
	})
	return nil
}
