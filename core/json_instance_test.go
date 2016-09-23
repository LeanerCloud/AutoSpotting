package autospotting

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testCase struct {
	instance        jsonInstance
	instanceType    string
	vCPU            int
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

func runTestCase(t *testing.T, tc testCase) {
	i := tc.instance

	if i.InstanceType != tc.instanceType {
		t.Error(tc.instanceType, "failed comparing instance type")
	}

	if i.VCPU != tc.vCPU {
		t.Error(tc.instanceType, "failed comparing CPU")
	}

	if i.Memory != tc.memory {
		t.Error(tc.instanceType, "failed comparing memory")
	}
	if i.Pricing["us-east-1"].Linux.OnDemand != tc.ondemandUSEast1 {
		t.Error(tc.instanceType, "failed comparing on-demand pricing")
	}

}
