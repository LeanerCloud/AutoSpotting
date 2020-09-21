// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

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

	data := []*ec2.SpotPrice{}
	err := ec2Conn.DescribeSpotPriceHistoryPages(params, func(page *ec2.DescribeSpotPriceHistoryOutput, lastPage bool) bool {
		data = append(data, page.SpotPriceHistory...)
		return true
	})

	if err != nil {
		logger.Println(s.conn.region, "Failed requesting spot prices:", err.Error())
		return err
	}

	s.data = data

	return nil
}
