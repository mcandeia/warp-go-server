// main_test.go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/_connect", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := New()
	serveHTTP := server.Routes().ServeHTTP

	rr := httptest.NewRecorder()

	serveHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

}

func TestRequestHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := New()
	serveHTTP := server.Routes().ServeHTTP

	rr := httptest.NewRecorder()

	serveHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// expected := "OK"
	// if rr.Body.String() != expected {
	// 	t.Errorf("handler returned unexpected body: got %v want %v",
	// 		rr.Body.String(), expected)
	// }
}
