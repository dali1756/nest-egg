package httpx

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type feeRequest struct {
	Fee string `json:"fee"`
}

var echoFee = Handler(func(w http.ResponseWriter, r *http.Request) error {
	var req feeRequest
	if err := DecodeJSON(r, &req); err != nil {
		return err
	}
	return WriteJSON(w, http.StatusOK, req)
})

func postBody(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
}

func TestDecodeJSONAcceptsAValidBody(t *testing.T) {
	rec, _ := serve(echoFee, postBody(`{"fee":"12.5"}`+"\n"))
	if rec.Code != http.StatusOK || rec.Body.String() != `{"fee":"12.5"}`+"\n" {
		t.Errorf("response = %d %s, want 200 {\"fee\":\"12.5\"}", rec.Code, rec.Body)
	}
}

func TestDecodeJSONRejectsBadBodies(t *testing.T) {
	for name, body := range map[string]string{
		"misspelled field": `{"fees":"0"}`,
		"trailing value":   `{"fee":"0"} {"fee":"1"}`,
		"trailing garbage": `{"fee":"0"} x`,
		"malformed":        `{"fee":`,
		"empty":            ``,
	} {
		t.Run(name, func(t *testing.T) {
			captureLogs(t)
			rec, id := serve(echoFee, postBody(body))
			want := fmt.Sprintf(`{"error":"invalid json","request_id":"%s"}`, id) + "\n"
			if rec.Code != http.StatusBadRequest || rec.Body.String() != want {
				t.Errorf("response = %d %s, want 400 %s", rec.Code, rec.Body, want)
			}
		})
	}
}

func TestDecodeJSONRejectsOversizedBodies(t *testing.T) {
	captureLogs(t)
	req := postBody(`{"fee":"` + strings.Repeat("9", 100) + `"}`)
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 32)

	echoFee.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || rec.Body.String() != `{"error":"request body too large"}`+"\n" {
		t.Errorf("response = %d %s, want 413 {\"error\":\"request body too large\"}", rec.Code, rec.Body)
	}
}
