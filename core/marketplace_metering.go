package autospotting

import (
	"errors"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/marketplacemetering"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// SSMParameterName stores the name of the SSM parameter that stores the success status of the latest metering call
const SSMParameterName = "autospotting-metering"

func meterMarketplaceUsage(savings float64) error {

	// Metering is supposed to be done from Fargate, but we check it here and return an error in case it failed before
	if RunningFromLambda() {
		log.Println("Running from Lambda")
		if failedFromFargate() {
			log.Println("Metering failed previously, exiting...")
			return errors.New("metering previously failed")
		}
		log.Println("Metering succeeded previously from Fargate, moving on...")
		return nil
	}

	mySession := session.Must(session.NewSession())

	// Create a MarketplaceMetering client with additional configuration
	svc := marketplacemetering.New(mySession, aws.NewConfig())

	charge := savings * 0.01 * as.config.SavingsCut
	units := int64(charge * 1000)

	log.Printf("Billing %v units for $%v saved/hour (%v%% of the generated savings of $%v/hour)",
		units, charge, as.config.SavingsCut, savings)

	res, err := svc.MeterUsage(&marketplacemetering.MeterUsageInput{
		ProductCode:    aws.String("9e5m3z5f5hlwdqcrv16xdi040"),
		Timestamp:      aws.Time(time.Now()),
		UsageDimension: aws.String("SavingsCut"),
		UsageQuantity:  aws.Int64(units),
	})

	if err != nil {
		log.Printf("Error submitting AWS Marketplace metering data: %v, received response: %v\n", err.Error(), res.String())
		markAsFailingFromFargate()
		return err
	}

	markAsSuccessfulFromFargate()
	return nil
}

func putSSMParameter(status string) {
	mySession := session.Must(session.NewSession())

	// Create a SSM client
	svc := ssm.New(mySession, aws.NewConfig().WithRegion("us-east-1"))

	_, err := svc.PutParameter(&ssm.PutParameterInput{
		Name:      aws.String(SSMParameterName),
		Overwrite: aws.Bool(true),
		Type:      aws.String("String"),
		Value:     aws.String(status),
	})
	if err != nil {
		log.Printf("Error persisting marketplace metering status(%s) to SSM: %s", status, err.Error())
	}
}

func markAsSuccessfulFromFargate() {
	putSSMParameter("success")
}

func markAsFailingFromFargate() {
	putSSMParameter("failure")
}

func failedFromFargate() bool {
	mySession := session.Must(session.NewSession())
	// Create a SSM client
	svc := ssm.New(mySession, aws.NewConfig().WithRegion("us-east-1"))
	res, err := svc.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(SSMParameterName),
	})

	if err != nil {
		log.Printf("Error reading marketplace metering status from SSM")
		if _, ok := err.(*ssm.ParameterNotFound); ok {
			log.Printf("Parameter not found: %v", err.Error())
			return false
		}
		log.Printf("Encountered error: %v", err.Error())
		return true
	}

	status := *res.Parameter.Value
	log.Printf("Retrieved marketplace metering status from SSM: %s", status)
	return status == "failure"
}
