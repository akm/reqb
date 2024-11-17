package testrequest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"

	"github.com/akm/go-testrequest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithServer(t *testing.T) {
	testServer := startEchoServer(t)
	testServer.Start()
	defer testServer.Close()

	testServerURL, err := url.Parse(testServer.URL)
	require.NoError(t, err)

	baseURL := testServer.URL

	defaultHeader := func() http.Header {
		return http.Header{
			"Accept-Encoding": []string{"gzip"},
			"User-Agent":      []string{"Go-http-client/1.1"},
		}
	}
	mergeHeader := func(h1, h2 http.Header) http.Header {
		for k, v := range h2 {
			h1[k] = v
		}
		return h1
	}
	expectedHeader := func(h http.Header) http.Header {
		return mergeHeader(defaultHeader(), h)
	}

	type pattern *struct {
		req      *http.Request
		expected *request
	}
	patterns := []pattern{
		{
			req: testrequest.GET(t, testrequest.BaseUrl(baseURL)),
			expected: &request{
				Method: http.MethodGet,
				Url:    "/",
				Header: expectedHeader(http.Header{}),
				Body:   "",
			},
		},
		{
			req: testrequest.POST(t,
				testrequest.BaseUrl(baseURL),
				testrequest.Path("/users"),
				testrequest.BodyString("hello, world"),
			),
			expected: &request{
				Method: http.MethodPost,
				Url:    "/users",
				Header: expectedHeader(http.Header{}),
				Body:   "hello, world",
			},
		},
		{
			req: testrequest.PUT(t,
				testrequest.BaseUrl(baseURL),
				testrequest.Path("/users/%d", 123),
				testrequest.BodyString("{\"name\":\"foo\"}"),
				testrequest.Header("Content-Type", "application/json"),
			),
			expected: &request{
				Method: http.MethodPut,
				Url:    "/users/123",
				Header: expectedHeader(http.Header{
					"Content-Type": []string{"application/json"},
				}),
				Body: "{\"name\":\"foo\"}",
			},
		},
		{
			req: testrequest.PATCH(t,
				testrequest.BaseUrl(baseURL),
				testrequest.Path("/users/%d", 123),
				testrequest.BodyBytes([]byte("{\"name\":\"bar\"}")),
				testrequest.Header("Content-Type", "application/json"),
				testrequest.Cookie(&http.Cookie{Name: "session", Value: "session1"}),
			),
			expected: &request{
				Method: http.MethodPatch,
				Url:    "/users/123",
				Header: expectedHeader(http.Header{
					"Content-Type": []string{"application/json"},
					"Cookie":       []string{"session=session1"},
				}),
				Body: "{\"name\":\"bar\"}",
			},
		},
		{
			req: testrequest.DELETE(t,
				testrequest.BaseUrl(baseURL),
				testrequest.Path("/users/%d", 456),
				testrequest.BodyString(""),
			),
			expected: &request{
				Method: http.MethodDelete,
				Url:    "/users/456",
				Header: expectedHeader(http.Header{
					"Cookie": []string{"session=session1"}, // from previous request
				}),
				Body: "",
			},
		},
		{
			req: testrequest.OPTIONS(t,
				// testrequest.BaseUrl(baseURL),
				testrequest.Scheme("http"),
				testrequest.Host(testServerURL.Hostname()),
				testrequest.PortString(testServerURL.Port()),
			),
			expected: &request{
				Method: http.MethodOptions,
				Url:    "/",
				Header: expectedHeader(http.Header{
					"Cookie": []string{"session=session1"}, // from previous request
				}),
				Body: "",
			},
		},
	}

	client := &http.Client{}
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client.Jar = jar
	for _, p := range patterns {
		t.Run(fmt.Sprintf("%s %s", p.req.Method, p.req.URL.Path), func(t *testing.T) {
			resp, err := client.Do(p.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("unexpected status code: %d", resp.StatusCode)
			}
			cookies := resp.Cookies()
			t.Logf("CLIENT %d cookies: %+v", len(cookies), cookies)
			client.Jar.SetCookies(p.req.URL, resp.Cookies())

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			t.Logf("CLIENT: respBody%s", string(respBody))

			var actual request
			if err := json.Unmarshal(respBody, &actual); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assert.Equal(t, p.expected, &actual)
		})
	}
}
