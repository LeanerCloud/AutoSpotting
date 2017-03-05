package ec2instancesinfo

import "testing"

func TestData(t *testing.T) {
	tests := []struct {
		name     string
		instance jsonInstance
		wantErr  bool
	}{{
		name: "Parsing t2.nano memory and price",
		instance: jsonInstance{
			InstanceType: "t2.nano",
			Memory:       0.5,
			Pricing: map[string]regionPrices{

				"us-east-1": {
					Linux{
						OnDemand: "0.0059"},
				},
			},
		},
		wantErr: false,
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Data()
			if (err != nil) != tt.wantErr {
				t.Errorf("Data() error = %v, wantErr %v", err, tt.wantErr)
			}

			for _, i := range *got {
				if i.InstanceType != tt.instance.InstanceType {
					continue
				}

				if i.Memory != tt.instance.Memory {
					t.Errorf("Data(): %v, want memory %v, got %v",
						tt.instance.InstanceType, tt.instance.Memory, i.Memory)
				}

				if i.Pricing["us-east-1"].Linux.OnDemand != tt.instance.Pricing["us-east-1"].Linux.OnDemand {
					t.Errorf("Data(): %v, want price %v, got %v",
						tt.instance.InstanceType,
						tt.instance.Pricing["us-east-1"].Linux.OnDemand,
						i.Pricing["us-east-1"].Linux.OnDemand)
				}
			}
		})
	}
}
