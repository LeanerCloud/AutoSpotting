package autospotting

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testCase struct {
	instance        JsonInstances
	index           int
	instanceType    string
	vcpu            int
	memory          float32
	ondemandUSEast1 string
}

func httpMock(bodyFileName string) *httptest.Server {
	return httptest.NewServer(
		http.HandlerFunc(
			func(
				w http.ResponseWriter,
				r *http.Request) {
				body, _ := ioutil.ReadFile(bodyFileName)
				fmt.Fprint(w, string(body))
			}),
	)
}

func runTestCase(t *testing.T, tc test_case) {
	i := tc.instance

	if i[tc.index].Instance_type != tc.instanceType {
		t.Error(tc.instanceType, "failed comparing instance type")
	}

	if i[tc.index].Vcpu != tc.vcpu {
		t.Error(tc.instanceType, "failed comparing CPU")
	}

	if i[tc.index].Memory != tc.memory {
		t.Error(tc.instanceType, "failed comparing memory")
	}
	if i[tc.index].Pricing["us-east-1"].Linux.Ondemand != tc.ondemandUSEast1 {
		t.Error(tc.instanceType, "failed comparing on-demand pricing")
	}

}

func testLoadFromURL(t *testing.T) {
	dataFile := "test_data/json_instance/instances.json"

	ts := httpMock(dataFile)
	defer ts.Close()

	var i JsonInstances
	i.loadFromURL(ts.URL)

	testCases := []testCase{
		// i, index, instanceType, numCPU, RAM, onDemandUSEast
		{i, 14, "t1.micro", 1, 0.613, "0.02"},
	}

	for _, tc := range test_cases {
		run_test_case(t, tc)
	}

}
