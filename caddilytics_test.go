package caddilytics

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mholt/caddy"
)

var reUUID = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$")

var setupTests = []struct {
	config string
	err    error
}{
	{
		"caddilytics UA-1234-5 test-session",
		nil,
	},
	{
		"UA-1234-5 test-session",
		errors.New("Testfile:1 - Parse error: Not Enough Arguments For Caddilytics"),
	},
	{
		"caddilytics UA-123-5 test-session",
		errors.New("Testfile:1 - Parse error: Not A Valid Tracking ID (UA-XXXX-Y): UA-123-5"),
	},
	{
		"caddilytics UA-1234-5 bro=ken",
		errors.New("Testfile:1 - Parse error: Not A Valid Session Cookie Name: bro=ken"),
	},
}

func TestSetup(t *testing.T) {
	for _, tt := range setupTests {
		c := caddy.NewTestController("http", tt.config)
		if err := setup(c); !reflect.DeepEqual(tt.err, err) {
			t.Errorf("Expected error '%v', got '%v'", tt.err, err.Error())
		}
	}
}

func TestServeHTTP(t *testing.T) {
	th := &testHelper{}
	ha := NewHandler("UA-1234-5", "test-session", th)
	ha.client = th
	r := httptest.NewRequest(
		"POST",
		"http://example.com/example?a=b",
		strings.NewReader(""),
	)
	r.Header.Add("Accept-Language", "it")
	r.Header.Add("User-Agent", "firefox")
	r.Header.Add("Referer", "http://example.com/source")
	ha.ServeHTTP(
		httptest.NewRecorder(),
		r,
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

	th2 := &testHelper{code: 500, err: errors.New("Internal Server Error")}
	ha2 := NewHandler("UA-1234-5", "test-session", th2)
	ha2.client = th2
	r2 := httptest.NewRequest(
		"POST",
		"http://example.com/example?a=b",
		strings.NewReader(""),
	)
	r2.AddCookie(&http.Cookie{Name: "test-session", Value: "bfe7ee5b-f58e-44dc-a30a-7d4e52392079"})
	ha2.ServeHTTP(
		httptest.NewRecorder(),
		r2,
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
	code               int
	err                error
}

func (th *testHelper) ServeHTTP(http.ResponseWriter, *http.Request) (int, error) {
	th.nextCall++
	return th.code, th.err
}

func (th *testHelper) Post(target string, contentType string, body io.Reader) (*http.Response, error) {
	th.postCall++
	buf := &bytes.Buffer{}
	buf.ReadFrom(body)
	th.postData, _ = url.ParseQuery(buf.String())
	return nil, nil
}
