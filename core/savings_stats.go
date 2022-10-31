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
	SSMParameterNameTotalFormat    = "/autospotting/savings/%s/total"
	SSMParameterNameByASGFormat    = "/autospotting/savings/%s/%s/%s" // time_interval/region/asg_name, eg hourly/us-east1-1/my_asg
	SSMParameterNameLastReportName = "/autospotting/savings/report_timestamp"

	hourly = "hourly"

	previousDay   = "previous_day"
	previousWeek  = "previous_week"
	previousMonth = "previous_month"
	previousYear  = "previous_year"
)

type savingsStats struct {
	ps              parameterStore
	sns             snsiface.SNSAPI
	topic           string
	reportFrequency string
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

	log.Printf("Retrieved last hourly savings from SSM: %s, with timestamp %v",
		*val, timestamp)

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

func (s *savingsStats) updateSavingsStats(savings float64) {
	s.persistTotalHourlySavings(savings)
	s.persistPreviousDayTotalSavings()
	s.persistPreviousWeekTotalSavings()
	s.persistPreviousMonthTotalSavings()
	s.persistPreviousYearTotalSavings()

	s.reportSavingsStats()
}

func (s *savingsStats) shouldReportSavingsStats() bool {

	if s.reportFrequency == "never" {
		log.Println("Savings Reports are disabled.")
		return false
	}

	_, lastReportTimestamp, err := s.ps.getSSMParameter(SSMParameterNameLastReportName)
	if err != nil {
		s.ps.putSSMParameter(SSMParameterNameLastReportName, time.Now().Format("Mon, 2 Jan 2006 15:04"))
		return false
	}

	if s.reportFrequency == "daily" {
		if lastReportTimestamp.YearDay() == time.Now().Local().YearDay() {
			log.Println("Using daily frequency and already sent a savings report today")
			return false
		}
		return true

	} else if s.reportFrequency == "weekly" {
		lry, lrw := lastReportTimestamp.ISOWeek()
		cy, cw := time.Now().Local().ISOWeek()
		if lry == cy && lrw == cw {
			log.Println("Using weekly frequency and already sent a savings report this week")
			return false
		}
		return true

	} else if s.reportFrequency == "monthly" {
		if lastReportTimestamp.Month() == time.Now().Local().Month() {
			log.Println("Using monthly frequency and already sent a savings report this month")
			return false
		}
		return true
	}

	log.Println("Report frequency misconfigured, exiting...")
	return false
}

func (s *savingsStats) reportSavingsStats() {

	if !s.shouldReportSavingsStats() {
		return
	}

	log.Println("SNS Topic:", s.topic)

	msg := fmt.Sprintf(`
  Hi there,

  Here's your %s AutoSpotting Savings Report.

  Savings...
  - today so far: $%.2f
  - yesterday: $%.2f
  - this week so far: $%.2f
  - last week: $%.2f
  - this month so far: $%.2f
  - last month: $%.2f
  - this year so far: $%.2f
  - all time $%.2f

  You may share these results freely within your company by forwarding this email, since
  management tends to love such reports. Also feel free to share such reports with
  us or post them on social media, it makes my day when I see those :-)

  I'd also love to hear from you if you have any questions, issues or feedback about AutoSpotting:
   - slack: https://join.slack.com/t/cloudutil/shared_invite/zt-xodcoi9j-1IcxNozXx1OW0gh_N08sjg
   - email: contact@autospotting.io

  Cheers,
  Cristian - the author of AutoSpotting

  PS:
  - You may also want to check out the other tools from our CloudUtil portfolio:
  https://aws.amazon.com/marketplace/seller-profile?id=a7ef2f5c-28b4-4dc2-90f5-ce4da9127c7f

  - We can also help you expand your AutoSpotting rollout or in general with cost optimization work
  unrelated to AutoSpotting.

    `,
		s.reportFrequency,
		s.calculateDaySoFarTotalSavings(),
		s.calculatePreviousDayTotalSavings(),
		s.calculateWeekSoFarTotalSavings(),
		s.calculatePreviousWeekTotalSavings(),
		s.calculateMonthSoFarTotalSavings(),
		s.calculatePreviousMonthTotalSavings(),
		s.calculateYearSoFarTotalSavings(),
		s.calculateAllTotalSavings())

	s.sns.Publish(
		&sns.PublishInput{
			Message:  aws.String(msg),
			Subject:  aws.String("AutoSpotting savings report - " + time.Now().Format("Mon, 2 Jan 2006 15:04")),
			TopicArn: aws.String(s.topic),
		},
	)
	s.ps.putSSMParameter(SSMParameterNameLastReportName, time.Now().Format("Mon, 2 Jan 2006 15:04"))
}
