package autospotting

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
)

// MeteringSSMParameterName stores the name of the SSM parameter that stores the success status of the latest metering call
const (
	SSMParameterNameTotalFormat = "/autospotting/savings/%s/total"
	SSMParameterNameByASGFormat = "/autospotting/savings/%s/%s/%s" // time_interval/region/asg_name, eg hourly/us-east1-1/my_asg

	hourly = "hourly"

	previousDay   = "previous_day"
	previousWeek  = "previous_week"
	previousMonth = "previous_month"
	previousYear  = "previous_year"
)

type savingsStats struct {
	ps    parameterStore
	sns   snsiface.SNSAPI
	topic string
}

func (s *savingsStats) calculateSavingsFromParameterHistory(key string, matchesTimeFilter func(t *time.Time) bool) float64 {

	var savings float64

	res, err := s.ps.getSSMParameterHistory(key)
	if err != nil {
		debug.Printf("Couldn't calculate daily savings so far: %v", err)
		return 0
	}

	for _, item := range res {

		if matchesTimeFilter(item.LastModifiedDate) {
			s, err := strconv.ParseFloat(*item.Value, 64)

			if err != nil {
				log.Printf("Encountered error: %v while parsing parameter value '%s' to float64", err.Error(), *item.Value)
				return 0
			}
			savings += s
		}
	}
	return savings
}

func (s *savingsStats) calculateDaySoFarTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, hourly)

	return s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {

		return t.Local().YearDay() == time.Now().YearDay()
	})
}

func (s *savingsStats) calculatePreviousDayTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, hourly)

	return s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
		return t.Local().YearDay() == time.Now().YearDay()-1 || t.Local().YearDay() > time.Now().YearDay()
	})
}

func (s *savingsStats) calculateWeekSoFarTotalSavings() float64 {

	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousDay)

	return s.calculateDaySoFarTotalSavings() +
		s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
			_, pw := t.Local().ISOWeek()
			_, w := time.Now().ISOWeek()
			return pw == w
		})
}

func (s *savingsStats) calculatePreviousWeekTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousDay)

	return s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
		_, pw := t.Local().ISOWeek()
		_, w := time.Now().ISOWeek()
		return pw == w-1 || pw > w
	})
}

func (s *savingsStats) calculateMonthSoFarTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousDay)

	return s.calculateDaySoFarTotalSavings() +
		s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
			return t.Local().Month() == time.Now().Month()
		})
}
func (s *savingsStats) calculatePreviousMonthTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousDay)

	return s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
		return t.Local().Month() == time.Now().Month()-1 || t.Local().Month() > time.Now().Month()
	})
}

func (s *savingsStats) calculateYearSoFarTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousMonth)

	return s.calculateMonthSoFarTotalSavings() +
		s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
			return t.Local().Year() == time.Now().Year()
		})
}

func (s *savingsStats) calculatePreviousYearTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousMonth)

	return s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
		return t.Local().Year() == time.Now().Year()-1 || t.Local().Year() > time.Now().Year()
	})
}

func (s *savingsStats) calculateAllTotalSavings() float64 {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousYear)

	return s.calculateYearSoFarTotalSavings() +
		s.calculateSavingsFromParameterHistory(param, func(t *time.Time) bool {
			return true
		})
}

func (s *savingsStats) existingPreviousDayTotalSavings() bool {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousDay)
	val, timestamp, err := s.ps.getSSMParameter(param)
	if err != nil {
		return false
	}

	log.Printf("Retrieved daily savings from SSM: %s", *val)
	return timestamp.Local().YearDay() == time.Now().YearDay()
}

func (s *savingsStats) existingPreviousWeekTotalSavings() bool {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousWeek)
	val, timestamp, err := s.ps.getSSMParameter(param)
	if err != nil {
		return false
	}

	log.Printf("Retrieved last week savings from SSM: %s", *val)
	_, pw := timestamp.Local().ISOWeek()
	_, w := time.Now().ISOWeek()
	return pw == w-1 || pw > w

}

func (s *savingsStats) existingCurrentHourTotalSavings() bool {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, hourly)
	val, timestamp, err := s.ps.getSSMParameter(param)
	if err != nil {
		return false
	}

	log.Printf("Retrieved last hourly savings from SSM: %s, with timestamp hour %d and current hour %d",
		*val, timestamp.Local().Hour(), time.Now().Hour())

	return timestamp.Local().Hour() == time.Now().Hour()
}

func (s *savingsStats) persistTotalHourlySavings(savings float64) {
	if s.existingCurrentHourTotalSavings() {
		log.Println("Current hour saving figure was already set, skipping calculation")
		return
	}
	log.Printf("Calculated hourly savings %f", savings)
	if savings < 0.00001 {
		return
	}
	param := fmt.Sprintf(SSMParameterNameTotalFormat, hourly)
	s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))
}

func (s *savingsStats) persistHourlyASGSavings(region string, asgSavings map[string]float64) {
	//for asg, savings := range asgSavings {
	// if s.existingCurrentHourTotalSavings() {
	//   log.Println("Current hour savings was already set, skipping calculation")
	//   return
	// }
	// if savings < 0.001 {
	//   return
	// }
	// param := fmt.Sprintf(SSMParameterNameTotalFormat, hourly)
	// s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))

	//s.ps.putSSMParameter(fmt.Sprintf(SSMParameterNameByASGFormat, "hourly", region, asg), fmt.Sprintf("%f", savings))
	//	}
}
func (s *savingsStats) persistPreviousDayTotalSavings() {

	if s.existingPreviousDayTotalSavings() {
		log.Println("Daily savings was already set today, skipping calculation")
		return
	}
	savings := s.calculatePreviousDayTotalSavings()
	log.Printf("Calculated previous day total savings %f", savings)
	if savings == 0 {
		return
	}
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousDay)
	s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))
}

func (s *savingsStats) persistPreviousWeekTotalSavings() {

	if s.existingPreviousWeekTotalSavings() {
		log.Println("Previous week savings was already set, skipping calculation")
		return
	}
	savings := s.calculatePreviousWeekTotalSavings()
	log.Printf("Calculated previous week total savings %f", savings)
	if savings == 0 {
		return
	}
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousWeek)
	s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))
}

func (s *savingsStats) existingPreviousMonthTotalSavings() bool {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousMonth)
	val, timestamp, err := s.ps.getSSMParameter(param)
	if err != nil {
		return false
	}

	log.Printf("Retrieved previous month savings from SSM: %s", *val)
	return timestamp.Local().Month() == time.Now().Month()
}

func (s *savingsStats) persistPreviousMonthTotalSavings() {

	if s.existingPreviousMonthTotalSavings() {
		log.Println("Previous month savings was already set this month, skipping calculation")
		return
	}
	savings := s.calculatePreviousMonthTotalSavings()
	log.Printf("Calculated previous month total savings %f", savings)
	if savings == 0 {
		return
	}
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousMonth)
	s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))
}

func (s *savingsStats) existingPreviousYearTotalSavings() bool {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousYear)
	val, timestamp, err := s.ps.getSSMParameter(param)
	if err != nil {
		return false
	}

	log.Printf("Retrieved previous year savings from SSM: %s", *val)
	return timestamp.Local().Year() == time.Now().Year()
}

func (s *savingsStats) persistPreviousYearTotalSavings() {

	if s.existingPreviousYearTotalSavings() {
		log.Println("Previous month savings was already set this month, skipping calculation")
		return
	}
	savings := s.calculatePreviousYearTotalSavings()
	log.Printf("Calculated previous year total savings %f", savings)
	if savings == 0 {
		return
	}
	param := fmt.Sprintf(SSMParameterNameTotalFormat, previousYear)
	s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))
}

func (s *savingsStats) updateAndReportSavingsStats(savings float64) {
	s.persistTotalHourlySavings(savings)
	s.persistPreviousDayTotalSavings()
	s.persistPreviousWeekTotalSavings()
	s.persistPreviousMonthTotalSavings()
	s.persistPreviousYearTotalSavings()

	log.Println("SNS Topic:", s.topic)

	msg := fmt.Sprintf(`AutoSpotting Savings report:
    - saved today so far: %f
    - saved this week so far: %f
    - saved this month so far: %f
    - saved this year so far: %f,
    - saved all time %f

    Please feel free to reach out to us at https://join.slack.com/t/cloudutil/shared_invite/zt-xodcoi9j-1IcxNozXx1OW0gh_N08sjg
    if you have any issues, questions or feedback about AutoSpotting.`,
		s.calculateDaySoFarTotalSavings(),
		s.calculateWeekSoFarTotalSavings(),
		s.calculateMonthSoFarTotalSavings(),
		s.calculateYearSoFarTotalSavings(),
		s.calculateAllTotalSavings())

	s.sns.Publish(
		&sns.PublishInput{
			Message:  aws.String(msg),
			Subject:  aws.String("AutoSpotting savings report - " + time.Now().Format("Mon, 2 Jan 2006 15:04")),
			TopicArn: aws.String(s.topic),
		},
	)

}
