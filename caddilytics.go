package caddilytics

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyhttp/httpserver"
	uuid "github.com/satori/go.uuid"
)

var (
	// reTrackingID must match provided Tracking IDs/Web Property IDs.
	reTrackingID = regexp.MustCompile("\\AUA-\\d{4,10}-\\d{1,4}\\z")
	// reSessionCookieName must NOT match provided session cookie name.
	reSessionCookieName = regexp.MustCompile("[\\s;,=]")
)

func init() {
	caddy.RegisterPlugin("caddilytics", caddy.Plugin{
		ServerType: "http",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	args := [3]string{}
	for i := 0; i < 3; i++ {
		if !c.Next() {
			return c.Err("Not Enough Arguments For Caddilytics")
		}
		args[i] = c.Val()
	}
	if !reTrackingID.MatchString(args[1]) {
		return c.Errf("Not A Valid Tracking ID (UA-XXXX-Y): %s", args[1])
	}
	if reSessionCookieName.MatchString(args[2]) {
		return c.Errf("Not A Valid Session Cookie Name: %s", args[2])
	}

	cfg := httpserver.GetConfig(c)
	mid := func(next httpserver.Handler) httpserver.Handler {
		return NewHandler(args[1], args[2], next)
	}
	cfg.AddMiddleware(mid)
	return nil
}

// Handler contains all functionality needed for measurement API calls.
type Handler struct {
	sessionCookieName string
	client            poster
	next              httpserver.Handler
	// prefix caches the parts of the POST body that do not change with every request.
	prefix string
}

type poster interface {
	Post(string, string, io.Reader) (*http.Response, error)
}

// NewHandler initializes a handler with a new http client (timeout 1 second).
func NewHandler(trackingID, sessionCookieName string, next httpserver.Handler) Handler {
	cl := &http.Client{Timeout: 1 * time.Second}
	prefix := url.Values{}
	prefix.Set("v", "1")
	prefix.Set("t", "pageview")
	prefix.Set("tid", trackingID)
	return Handler{sessionCookieName, cl, next, prefix.Encode()}
}

func (ha Handler) setCookie(w http.ResponseWriter) string {
	clientID := uuid.NewV4().String()
	http.SetCookie(w, &http.Cookie{
		Expires:  time.Unix(2147483647, 0),
		Name:     ha.sessionCookieName,
		Value:    clientID,
		Secure:   true,
		HttpOnly: true,
	})
	return clientID
}

func (ha Handler) ensureCookie(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie(ha.sessionCookieName)
	if err != nil {
		return ha.setCookie(w)
	}
	clientID := cookie.Value
	if clientID == "" {
		return ha.setCookie(w)
	}
	return clientID
}

func (ha Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) (code int, err error) {
	clientID := ha.ensureCookie(w, r)
	code, err = ha.next.ServeHTTP(w, r)
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
		trackingData.Set("exf", err.Error())
	}
	go ha.client.Post(
		"http://www.google-analytics.com/collect",
		"application/x-www-form-urlencoded; charset=UTF-8",
		strings.NewReader(ha.prefix+"&"+trackingData.Encode()),
	)
	return
}
