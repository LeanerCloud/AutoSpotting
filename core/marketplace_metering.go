package autospotting

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/marketplacemetering"
)

func meterMarketplaceUsage(savings float64) error {
	mySession := session.Must(session.NewSession())

	// Create a MarketplaceMetering client with additional configuration
	svc := marketplacemetering.New(mySession, aws.NewConfig().WithRegion("us-east-1"))

	res, err := svc.MeterUsage(&marketplacemetering.MeterUsageInput{
		ProductCode:    aws.String("9e5m3z5f5hlwdqcrv16xdi040"),
		Timestamp:      aws.Time(time.Now()),
		UsageDimension: aws.String("SavingsCut"),
		UsageQuantity:  aws.Int64(int64(savings * 1000 * 0.05)),
	})

	if err != nil {
		fmt.Printf("Error submitting Marketplace metering data: %v, received response: %v\n", err.Error(), res.String())
		return err
	}
	return nil
}
