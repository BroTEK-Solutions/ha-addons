package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

const maxAlloyHTML = 4 << 20

func newAlloyProxy(rawURL string) (http.Handler, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		// HTML has to be rewritten for Home Assistant's tokenized ingress base.
		request.Header.Set("Accept-Encoding", "identity")
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if location := response.Header.Get("Location"); strings.HasPrefix(location, "/alloy/") {
			response.Header.Set("Location", "./"+strings.TrimPrefix(location, "/alloy/"))
		}
		if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, maxAlloyHTML+1))
		if err != nil {
			return fmt.Errorf("read Alloy HTML: %w", err)
		}
		_ = response.Body.Close()
		if len(body) > maxAlloyHTML {
			return fmt.Errorf("Alloy HTML exceeds %d bytes", maxAlloyHTML)
		}
		body = bytes.ReplaceAll(body, []byte(`href="/alloy/"`), []byte(`href="./"`))
		response.Body = io.NopCloser(bytes.NewReader(body))
		response.ContentLength = int64(len(body))
		response.Header.Set("Content-Length", strconv.Itoa(len(body)))
		response.Header.Del("ETag")
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Alloy is not ready", http.StatusBadGateway)
	}
	return proxy, nil
}
