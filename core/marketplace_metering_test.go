package autospotting

import (
	"testing"
	"time"
)

func Test_marketplaceMetering_calculateDiscount(t *testing.T) {
	specialAccountID := "255827576516"
	beforeApril2029, _ := time.Parse(time.RFC3339, "2029-03-31T23:59:59Z")
	afterApril2029, _ := time.Parse(time.RFC3339, "2029-04-01T00:00:00Z")

	type fields struct {
		accountID *string
	}
	type args struct {
		units       int32
		currentTime time.Time // Add this to simulate current time
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   int32
	}{
		{
			name: "No Account ID, No Discount",
			fields: fields{
				accountID: nil,
			},
			args: args{
				units:       100,
				currentTime: beforeApril2029,
			},
			want: 100,
		},
		{
			name: "Different Account ID, No Discount",
			fields: fields{
				accountID: stringPointer("123456789012"),
			},
			args: args{
				units:       300,
				currentTime: beforeApril2029,
			},
			want: 300,
		},
		{
			name: "Special Account ID, Units Below Threshold, No Units Remain, Before April 2029",
			fields: fields{
				accountID: stringPointer(specialAccountID),
			},
			args: args{
				units:       200,
				currentTime: beforeApril2029,
			},
			want: 0,
		},
		{
			name: "Special Account ID, Units Above Threshold, Discount Applied, Before April 2029",
			fields: fields{
				accountID: stringPointer(specialAccountID),
			},
			args: args{
				units:       500,
				currentTime: beforeApril2029,
			},
			want: 226, // 500 - 274
		},
		{
			name: "Special Account ID, After April 2029, No Discount",
			fields: fields{
				accountID: stringPointer(specialAccountID),
			},
			args: args{
				units:       500,
				currentTime: afterApril2029,
			},
			want: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &marketplaceMetering{
				accountID: tt.fields.accountID,
			}
			if got := m.calculateDiscount(tt.args.units, tt.args.currentTime); got != tt.want {
				t.Errorf("marketplaceMetering.calculateDiscount() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function to create a pointer to a string (for accountID)
func stringPointer(s string) *string {
	return &s
}
