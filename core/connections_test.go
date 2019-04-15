// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"testing"
)

func Test_connections_connect(t *testing.T) {

	tests := []struct {
		name   string
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
