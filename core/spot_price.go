package autospotting

import (
	"errors"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type spotPrices struct {
	data     []*ec2.SpotPrice
	conn     connections
	duration time.Duration
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

func (s *spotPrices) filterData(az string, instanceType string) []*ec2.SpotPrice {
	var r []*ec2.SpotPrice

	for _, p := range s.data {
		if p.AvailabilityZone != nil &&
			p.InstanceType != nil &&
			p.Timestamp != nil &&
			*p.AvailabilityZone == az &&
			*p.InstanceType == instanceType {
			r = append(r, p)
		}
	}
	return r
}

func (s *spotPrices) average(az string, instanceType string) (float64, error) {

	var sum int64

	data := s.filterData(az, instanceType)

	debug.Println(data)

	if len(data) == 0 {
		return -1, errors.New("Can't determine average, missing spot data")
	}

	if len(data) == 1 {

		price, err := strconv.ParseFloat(*data[0].SpotPrice, 64)
		if err != nil {
			return -1, err
		}
		return price, nil
	}

	// start with the first item
	prevTimestamp := *data[0].Timestamp
	prevPrice, _ := strconv.Atoi(*data[0].SpotPrice)

	for _, p := range data[0:] {

		timediff := (*p.Timestamp).Sub(prevTimestamp).Nanoseconds()

		sum += int64(prevPrice) * timediff

		debug.Println(prevTimestamp.String(), prevPrice, timediff, sum)

		prevPrice, _ = strconv.Atoi(*p.SpotPrice)
		prevTimestamp = *p.Timestamp
	}

	return float64(sum) / float64(s.duration.Nanoseconds()), nil
}
