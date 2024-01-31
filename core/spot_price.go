// Copyright (c) 2016-2022 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
)

type spotPrices struct {
	data []ec2Types.SpotPrice
	conn *connections
}

func (s *spotPrices) fetch(
	duration time.Duration,
	availabilityZone *string,
	instanceTypes []ec2Types.InstanceType) error {

	debug.Println(s.conn.region, "Requesting spot prices")

	params := &ec2.DescribeSpotPriceHistoryInput{
		StartTime:        aws.Time(time.Now().Add(-1 * duration)),
		EndTime:          aws.Time(time.Now()),
		AvailabilityZone: availabilityZone,
		InstanceTypes:    instanceTypes,
	}

	paginator := ec2.NewDescribeSpotPriceHistoryPaginator(s.conn.ec2, params)

	data := []ec2Types.SpotPrice{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			log.Println(s.conn.region, "Failed requesting spot prices:", err)
			return err
		}
		data = append(data, page.SpotPriceHistory...)
		debug.Printf("DescribeSpotPriceHistory lastPage: %v", !paginator.HasMorePages())
	}

	s.data = data

	return nil
}
