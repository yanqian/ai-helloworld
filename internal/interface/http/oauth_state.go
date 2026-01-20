package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthStateMaxAge     = 300
)

type oauthStateCookie struct {
	State        string `json:"state"`
	CodeVerifier string `json:"verifier"`
}

func setOAuthStateCookie(c *gin.Context, state, codeVerifier string) {
	payload := oauthStateCookie{State: state, CodeVerifier: codeVerifier}
	data, _ := json.Marshal(payload)
	encoded := base64.RawURLEncoding.EncodeToString(data)
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oauthStateCookieName, encoded, oauthStateMaxAge, "/", "", secure, true)
}

func clearOAuthStateCookie(c *gin.Context) {
	secure := c.Request.TLS != nil
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oauthStateCookieName, "", -1, "/", "", secure, true)
}

func readOAuthStateCookie(c *gin.Context) (oauthStateCookie, bool) {
	value, err := c.Cookie(oauthStateCookieName)
	if err != nil || value == "" {
		return oauthStateCookie{}, false
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return oauthStateCookie{}, false
	}
	var payload oauthStateCookie
	if err := json.Unmarshal(data, &payload); err != nil {
		return oauthStateCookie{}, false
	}
	if payload.State == "" || payload.CodeVerifier == "" {
		return oauthStateCookie{}, false
	}
	return payload, true
}
