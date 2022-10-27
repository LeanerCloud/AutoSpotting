package autospotting

import (
	"errors"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/marketplacemetering"
	"github.com/aws/aws-sdk-go/service/marketplacemetering/marketplacemeteringiface"
)

// MeteringSSMParameterName stores the name of the SSM parameter that stores the success status of the latest metering call
const MeteringSSMParameterName = "autospotting-metering"

type marketplaceMetering struct {
	mpSvc marketplacemeteringiface.MarketplaceMeteringAPI
	ps    parameterStore
	svs   savingsStats
}

func (m *marketplaceMetering) meterMarketplaceUsage(savings float64) error {

	// Metering is supposed to be done from Fargate, but we check it here and return an error in case it failed before
	if RunningFromLambda() {
		log.Println("Running from Lambda")
		if m.failedFromFargate() {
			log.Println("Metering failed previously, exiting...")
			return errors.New("metering previously failed")
		}
		log.Println("Metering succeeded previously from Fargate, moving on...")
		return nil
	}

	m.svs.updateAndReportSavingsStats(savings)

	charge := savings * 0.01 * as.config.SavingsCut
	units := int64(charge * 1000)

	if units == 0 {
		log.Printf("Your usage is within the free tier, yay!")
	}

	log.Printf("Billing %v units for $%f saved/hour (%v%% of the generated savings of $%f/hour)",
		units, charge, as.config.SavingsCut, savings)

	res, err := m.mpSvc.MeterUsage(&marketplacemetering.MeterUsageInput{
		ProductCode:    aws.String("9e5m3z5f5hlwdqcrv16xdi040"),
		Timestamp:      aws.Time(time.Now()),
		UsageDimension: aws.String("SavingsCut"),
		UsageQuantity:  aws.Int64(units),
	})

	if err != nil {
		log.Printf("Error submitting AWS Marketplace metering data: %v, received response: %v\n", err.Error(), res.String())
		m.markAsFailingFromFargate()
		return err
	}

	m.markAsSuccessfulFromFargate()
	return nil
}

func (m *marketplaceMetering) markAsSuccessfulFromFargate() {
	m.ps.putSSMParameter(MeteringSSMParameterName, "success")
}

func (m *marketplaceMetering) markAsFailingFromFargate() {
	m.ps.putSSMParameter(MeteringSSMParameterName, "failure")
}

func (m *marketplaceMetering) failedFromFargate() bool {
	val, _, err := m.ps.getSSMParameter(MeteringSSMParameterName)
	if err != nil && *val == "failure" {
		return true
	}
	return false
}
