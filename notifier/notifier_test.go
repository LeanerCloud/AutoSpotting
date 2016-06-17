package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type test_case struct {
	status          int
	body            string
	f               func(string) bool
	expected_result bool
}

func http_mock(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if body != "" {
			fmt.Fprintln(w, body)
		}
	}))
}

func run_http_test_case(t *testing.T, tc test_case) {
	ts := http_mock(tc.status, tc.body)
	defer ts.Close()

	result := tc.f(ts.URL)

	if result != tc.expected_result {
		t.Fail()
	}

}

func Test_Terminating(t *testing.T) {

	test_cases := []test_case{
		{200, "some time in the future", terminating, true},
		{200, "", terminating, true},
		{404, "not found", terminating, false},
		{502, "123", terminating, false},
	}
	for _, tc := range test_cases {
		run_http_test_case(t, tc)
	}
}
