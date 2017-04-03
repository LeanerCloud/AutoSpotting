package autospotting

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

func Test_connections_connect(t *testing.T) {
	type fields struct {
		session     *session.Session
		autoScaling autoscalingiface.AutoScalingAPI
		ec2         ec2iface.EC2API
		region      string
	}

	tests := []struct {
		name   string
		fields fields
		region string
		match  bool
	}{
		{
			name:   "connect to region foo",
			region: "foo",
			match:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &connections{}
			c.connect(tt.region)
			if (c.region == tt.region) != tt.match {
				t.Errorf("connections.connect() c.region = %v, expected %v",
					c.region, tt.region)
			}
		})
	}
}
