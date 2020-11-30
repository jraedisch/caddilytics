// Package caddilytics implements a minimal Caddy module for tracking HTTP requests via Google Analytics Measurement Protocol.
package caddilytics

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	// reTrackingID must match provided Tracking IDs/Web Property IDs.
	reTrackingID = regexp.MustCompile("\\AUA-\\d{4,10}-\\d{1,4}\\z")
	// reSessionCookieName must NOT match provided session cookie name.
	reSessionCookieName = regexp.MustCompile("[\\s;,=]")
)

func init() {
	caddy.RegisterModule(Middleware{})
	httpcaddyfile.RegisterHandlerDirective("caddilytics", parseCaddyfile)
}

type poster interface {
	Post(string, string, io.Reader) (*http.Response, error)
}

// Middleware implements an HTTP handler that ensures a session cookie and sends analytics data to Google.
type Middleware struct {
	TrackingID        string
	SessionCookieName string
	client            poster
	logger            *zap.Logger
	// prefix caches the parts of the POST body that do not change with every request.
	prefix string
}

// Provision implements caddy.Provisioner.
func (m *Middleware) Provision(ctx caddy.Context) error {
	m.logger = ctx.Logger(m)
	m.client = &http.Client{Timeout: 1 * time.Second}
	prefix := url.Values{}
	prefix.Set("v", "1")
	prefix.Set("t", "pageview")
	prefix.Set("tid", m.TrackingID)
	m.prefix = prefix.Encode()
	return nil
}

// Validate implements caddy.Validator.
func (m *Middleware) Validate() error {
	if !reTrackingID.MatchString(m.TrackingID) {
		return fmt.Errorf("not a valid tracking ID (UA-XXXX-Y): %s", m.TrackingID)
	}
	if reSessionCookieName.MatchString(m.SessionCookieName) {
		return fmt.Errorf("not a valid session cookie name: %s", m.SessionCookieName)
	}
	return nil
}

// CaddyModule returns the Caddy module information.
func (Middleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.caddilytics",
		New: func() caddy.Module { return new(Middleware) },
	}
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (m *Middleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	for d.Next() {
		if !d.Args(&m.TrackingID, &m.SessionCookieName) {
			return d.ArgErr()
		}
	}
	return nil
}

// parseCaddyfile unmarshals tokens from h into a new Middleware.
func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m Middleware
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// ServeHTTP ensures a session cookie before and sends tracking data asynchronously after calling next middleware.
func (m Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	clientID, uuidErr := m.ensureCookie(w, r)
	httpErr := next.ServeHTTP(w, r)
	if uuidErr != nil {
		m.logger.Error(fmt.Sprintf("uuid generation error: %v", uuidErr))
		return nil
	}

	var code int
	if handlerErr, ok := httpErr.(caddyhttp.HandlerError); ok {
		code = handlerErr.StatusCode
		httpErr = handlerErr.Err
	}
	trackingData := url.Values{}
	trackingData.Set("cid", clientID)
	trackingData.Set("dl", r.RequestURI)
	trackingData.Set("ds", "web")
	trackingData.Set("ua", r.UserAgent())
	trackingData.Set("uip", strings.Split(r.RemoteAddr, ":")[0])
	if dr := r.Referer(); dr != "" {
		trackingData.Set("dr", dr)
	}
	if lang := r.Header.Get("Accept-Language"); lang != "" {
		trackingData.Set("ul", lang)
	}
	if code > 499 {
		trackingData.Set("exf", httpErr.Error())
	}
	go m.client.Post(
		"http://www.google-analytics.com/collect",
		"application/x-www-form-urlencoded; charset=UTF-8",
		strings.NewReader(m.prefix+"&"+trackingData.Encode()),
	)
	return nil
}

func (m *Middleware) setCookie(w http.ResponseWriter) (string, error) {
	randomID, err := uuid.NewRandom()
	clientID := randomID.String()
	http.SetCookie(w, &http.Cookie{
		Expires:  time.Unix(2147483647, 0),
		Name:     m.SessionCookieName,
		Value:    clientID,
		Secure:   true,
		HttpOnly: true,
	})
	return clientID, err
}

func (m *Middleware) ensureCookie(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie(m.SessionCookieName)
	if err != nil {
		return m.setCookie(w)
	}
	clientID := cookie.Value
	if clientID == "" {
		return m.setCookie(w)
	}
	return clientID, nil
}

// Interface guards
var (
	_ caddy.Provisioner           = (*Middleware)(nil)
	_ caddy.Validator             = (*Middleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*Middleware)(nil)
)
