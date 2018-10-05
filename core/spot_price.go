package autospotting

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type spotPrices struct {
	data []*ec2.SpotPrice
	conn connections
}

// fetch queries all spot prices in the current region
func (s *spotPrices) fetch(product string,
	duration time.Duration,
	availabilityZone *string,
	instanceTypes []*string) error {

	logger.Println(s.conn.region, "Requesting spot prices")

	ec2Conn := s.conn.ec2
	params := &ec2.DescribeSpotPriceHistoryInput{
		ProductDescriptions: []*string{
			aws.String(product),
		},
		StartTime:        aws.Time(time.Now().Add(-1 * duration)),
		EndTime:          aws.Time(time.Now()),
		AvailabilityZone: availabilityZone,
		InstanceTypes:    instanceTypes,
	}

	resp, err := ec2Conn.DescribeSpotPriceHistory(params)

	if err != nil {
		logger.Println(s.conn.region, "Failed requesting spot prices:", err.Error())
		return err
	}

	s.data = resp.SpotPriceHistory

	return nil
}
