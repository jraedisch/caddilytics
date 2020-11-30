package caddilytics

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

var reUUID = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$")

func TestValidation(t *testing.T) {
	m := &Middleware{TrackingID: "UA-1234-5", SessionCookieName: "my_session"}
	err := m.Validate()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUnmarshalling(t *testing.T) {
	m := &Middleware{}
	err := m.UnmarshalCaddyfile(caddyfile.NewTestDispenser("caddilytics UA-1234-5 my_session"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if m.TrackingID != "UA-1234-5" {
		t.Errorf("unexpected TrackingID: %s", m.TrackingID)
	}
	if m.SessionCookieName != "my_session" {
		t.Errorf("unexpected SessionCookieName: %s", m.SessionCookieName)
	}
}

func TestServeHTTP(t *testing.T) {
	prefix := url.Values{}
	prefix.Set("v", "1")
	prefix.Set("t", "pageview")
	prefix.Set("tid", "UA-1234-5")
	prefixEncoded := prefix.Encode()

	th := &testHelper{}
	m := &Middleware{
		TrackingID:        "UA-1234-5",
		SessionCookieName: "test-session",
		client:            th,
		prefix:            prefixEncoded,
	}
	r := httptest.NewRequest(
		"POST",
		"http://example.com/example?a=b",
		strings.NewReader(""),
	)
	r.Header.Add("Accept-Language", "it")
	r.Header.Add("User-Agent", "firefox")
	r.Header.Add("Referer", "http://example.com/source")
	m.ServeHTTP(
		httptest.NewRecorder(),
		r,
		th,
	)

	time.Sleep(1 * time.Millisecond)
	if 1 != th.nextCall {
		t.Errorf("Expected next to be called once, got %d", th.nextCall)
	}
	if 1 != th.postCall {
		t.Errorf("Expected post to be called once, got %d", th.postCall)
	}
	if "http://example.com/example?a=b" != th.postData.Get("dl") {
		t.Errorf("Unexpected target '%s'", th.postData.Get("dl"))
	}
	if "UA-1234-5" != th.postData.Get("tid") {
		t.Errorf("Unexpected tracking ID '%s'", th.postData.Get("tid"))
	}
	if "it" != th.postData.Get("ul") {
		t.Errorf("Unexpected language '%s'", th.postData.Get("ul"))
	}
	if "firefox" != th.postData.Get("ua") {
		t.Errorf("Unexpected user agent '%s'", th.postData.Get("ua"))
	}
	if !reUUID.MatchString(th.postData.Get("cid")) {
		t.Errorf("Client ID is not a UUID: %s", th.postData.Get("cid"))
	}
	if "http://example.com/source" != th.postData.Get("dr") {
		t.Errorf("Unexpected referer: %s", th.postData.Get("dr"))
	}

	th2 := &testHelper{err: caddyhttp.Error(500, errors.New("Internal Server Error"))}
	m2 := &Middleware{
		TrackingID:        "UA-1234-5",
		SessionCookieName: "test-session",
		client:            th2,
		prefix:            prefixEncoded,
	}
	r2 := httptest.NewRequest(
		"POST",
		"http://example.com/example?a=b",
		strings.NewReader(""),
	)
	r2.AddCookie(&http.Cookie{Name: "test-session", Value: "bfe7ee5b-f58e-44dc-a30a-7d4e52392079"})
	m2.ServeHTTP(
		httptest.NewRecorder(),
		r2,
		th2,
	)

	time.Sleep(1 * time.Millisecond)
	if "bfe7ee5b-f58e-44dc-a30a-7d4e52392079" != th2.postData.Get("cid") {
		t.Errorf("Unexpected client ID '%s'", th2.postData.Get("cid"))
	}
	if "Internal Server Error" != th2.postData.Get("exf") {
		t.Errorf("Unexpected error message '%s'", th2.postData.Get("exf"))
	}
}

type testHelper struct {
	postCall, nextCall int
	postData           url.Values
	err                error
}

func (th *testHelper) ServeHTTP(http.ResponseWriter, *http.Request) error {
	th.nextCall++
	return th.err
}

func (th *testHelper) Post(target string, contentType string, body io.Reader) (*http.Response, error) {
	th.postCall++
	buf := &bytes.Buffer{}
	buf.ReadFrom(body)
	th.postData, _ = url.ParseQuery(buf.String())
	return nil, nil
}
