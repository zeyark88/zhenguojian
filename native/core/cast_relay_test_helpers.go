package core

import (
	"net/http"
	"net/http/httptest"
)

func httptestNewRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func newRelayRequest(path string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:1"+path, nil)
	return request
}

func relayRecorderResult(r *httptest.ResponseRecorder) (int, string) { return r.Code, r.Body.String() }
