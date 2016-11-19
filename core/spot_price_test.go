package autospotting

import (
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const TOLERANCE = 0.000001

func Test_spotPrices_average(t *testing.T) {
	NOW := time.Now()

	type fields struct {
		data     []*ec2.SpotPrice
		conn     connections
		duration time.Duration
	}
	tests := []struct {
		name    string
		fields  fields
		want    float64
		wantErr bool
		az      string

		inst string
	}{
		{name: "Empty data would return an error",
			fields: fields{data: []*ec2.SpotPrice{},
				conn:     connections{},
				duration: time.Minute},
			want:    -1,
			wantErr: true,
		},

		{name: "Data from last minute",
			fields: fields{data: []*ec2.SpotPrice{
				{
					SpotPrice:        aws.String("10"),
					AvailabilityZone: aws.String("us-east-1a"),
					Timestamp:        aws.Time(NOW.Add(-1 * time.Minute)),
					InstanceType:     aws.String("c3.xlarge"),
				},
				{
					SpotPrice:        aws.String("2"),
					AvailabilityZone: aws.String("us-east-1a"),
					Timestamp:        aws.Time(NOW.Add(-1 * time.Minute)),
					InstanceType:     aws.String("c3.large"),
				},
			}, conn: connections{},
				duration: time.Minute},
			az:      "us-east-1a",
			inst:    "c3.large",
			want:    2,
			wantErr: false,
		},

		{name: "Easy price average over the last hour",
			fields: fields{data: []*ec2.SpotPrice{
				{
					SpotPrice:        aws.String("1"),
					Timestamp:        aws.Time(NOW.Add(-1 * time.Hour)),
					AvailabilityZone: aws.String("us-east-1a"),
					InstanceType:     aws.String("c3.large"),
				},
				{
					SpotPrice:        aws.String("2"),
					Timestamp:        aws.Time(NOW.Add(-40 * time.Minute)),
					AvailabilityZone: aws.String("us-east-1a"),
					InstanceType:     aws.String("c3.large"),
				},
				{
					SpotPrice:        aws.String("3"),
					Timestamp:        aws.Time(NOW.Add(-20 * time.Minute)),
					AvailabilityZone: aws.String("us-east-1a"),
					InstanceType:     aws.String("c3.large"),
				},
				{
					SpotPrice:        aws.String("4"),
					Timestamp:        aws.Time(NOW),
					AvailabilityZone: aws.String("us-east-1a"),
					InstanceType:     aws.String("c3.large"),
				},
				{
					SpotPrice:        aws.String("29"),
					Timestamp:        aws.Time(NOW.Add(-40 * time.Minute)),
					AvailabilityZone: aws.String("us-east-1a"),
					InstanceType:     aws.String("c3.xlarge"),
				},
			}, conn: connections{},
				duration: time.Hour,
			},
			az:      "us-east-1a",
			inst:    "c3.large",
			want:    2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s := &spotPrices{
				data:     tt.fields.data,
				conn:     tt.fields.conn,
				duration: tt.fields.duration,
			}
			got, err := s.average(tt.az, tt.inst)
			if (err != nil) != tt.wantErr {
				t.Errorf("spotPrices.average() error = %v, wantErr %v", err, tt.wantErr)

			}
			if diff := math.Abs(got - tt.want); diff > TOLERANCE {
				t.Errorf("spotPrices.average() = %v, want %v", got, tt.want)
			}
		})
	}
}
