package autospotting

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/sns/snsiface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/jinzhu/now"
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
	history         *savingsHistory
}

type savingsHistory struct {
	hourlyHistory, dailyHistory, weeklyHistory, monthlyHistory, yearlyHistory []*ssm.ParameterHistory
}

func (s *savingsStats) loadHistory(timeline string) []*ssm.ParameterHistory {
	history, err := s.ps.getSSMParameterHistory(fmt.Sprintf(SSMParameterNameTotalFormat, timeline))
	if err != nil {
		debug.Printf("Couldn't get %s history: %v", timeline, err)
	}
	return history
}

func (s *savingsStats) loadAllHistory() {

	s.history = &savingsHistory{
		hourlyHistory:  s.loadHistory(hourly),
		dailyHistory:   s.loadHistory(previousDay),
		weeklyHistory:  s.loadHistory(previousWeek),
		monthlyHistory: s.loadHistory(previousMonth),
		yearlyHistory:  s.loadHistory(previousYear),
	}
}

func (s *savingsStats) calculateSavingsFromHistory(history []*ssm.ParameterHistory, matchesTimeFilter func(t *time.Time) bool) float64 {

	var savings float64

	for _, item := range history {
		debug.Printf("Processing Savings from History item of timestamp %v with value %v",
			item.LastModifiedDate, *item.Value)

		if matchesTimeFilter(item.LastModifiedDate) {

			s, err := strconv.ParseFloat(*item.Value, 64)

			if err != nil {
				debug.Printf("Encountered error: %v while parsing parameter value '%s' to float64", err.Error(), *item.Value)
				return 0
			}
			savings += s
			debug.Printf("Savings History item: %v matches the time filter, added to savings, current savings figure %v",
				*item.Value, savings)

		} else {
			debug.Printf("Savings History item: %v does not match the time filter", *item.Value)
		}
	}
	return savings
}

func (s *savingsStats) calculateDaySoFarTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.hourlyHistory, func(t *time.Time) bool {
		ret := t.Local().After(now.BeginningOfDay())
		debug.Printf("Checking if timestamp %v is from today: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculatePreviousDayTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.hourlyHistory, func(t *time.Time) bool {
		ret := t.Local().After(now.BeginningOfDay().AddDate(0, 0, -1)) &&
			t.Local().Before(now.BeginningOfDay())
		debug.Printf("Checking if timestamp %v is from yesterday: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateWeekSoFarTotalSavings() float64 {
	return s.calculateDaySoFarTotalSavings() +
		s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
			ret := t.Local().After(now.BeginningOfWeek())
			debug.Printf("Checking if timestamp %v is from this week: %v\n", t.Local(), ret)
			return ret
		})
}

func (s *savingsStats) calculatePreviousWeekTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
		ret := t.Local().After(now.BeginningOfWeek().AddDate(0, 0, -7)) &&
			t.Local().Before(now.BeginningOfWeek())
		debug.Printf("Checking if timestamp %v is from last week: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateMonthSoFarTotalSavings() float64 {
	return s.calculateDaySoFarTotalSavings() +
		s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
			ret := t.Local().After(now.BeginningOfMonth())
			debug.Printf("Checking if timestamp %v is from this month: %v\n", t.Local(), ret)
			return ret
		})
}

func (s *savingsStats) calculatePreviousMonthTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
		ret := t.Local().After(now.BeginningOfMonth().AddDate(0, -1, 0)) &&
			t.Local().Before(now.BeginningOfMonth())
		debug.Printf("Checking if timestamp %v is from last month: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateYearSoFarTotalSavings() float64 {
	return s.calculateMonthSoFarTotalSavings() +
		s.calculateSavingsFromHistory(s.history.monthlyHistory, func(t *time.Time) bool {
			ret := t.Local().Year() == time.Now().Year()
			debug.Printf("Checking if timestamp %v is from this year: %v\n", t.Local(), ret)
			return ret
		})
}

func (s *savingsStats) calculatePreviousYearTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.monthlyHistory, func(t *time.Time) bool {
		ret := t.Local().Year() == time.Now().Year()-1
		debug.Printf("Checking if timestamp %v is from last year: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateAllTotalSavings() float64 {
	return s.calculateYearSoFarTotalSavings() +
		s.calculateSavingsFromHistory(s.history.yearlyHistory, func(t *time.Time) bool {
			return true
		})
}

func (s *savingsStats) existingTotalSavings(timeline string, matchesTimeFilter func(t *time.Time) bool) bool {
	param := fmt.Sprintf(SSMParameterNameTotalFormat, timeline)
	debug.Printf("Checking total savings for path %s\n", param)
	val, timestamp, err := s.ps.getSSMParameter(param)
	if err != nil {
		debug.Printf("Missing parameter for path %s, %s\n", param, err.Error())
		return false
	}

	recentEnough := matchesTimeFilter(timestamp)
	log.Printf("Retrieved %s savings from SSM: %s, ignore: %v", timeline, *val, !recentEnough)
	return recentEnough
}

func (s *savingsStats) existingCurrentHourTotalSavings() bool {
	return s.existingTotalSavings(hourly, func(t *time.Time) bool {
		return t.Local().Hour() == time.Now().Hour()
	})
}

func (s *savingsStats) existingPreviousDayTotalSavings() bool {
	return s.existingTotalSavings(previousDay, func(t *time.Time) bool {
		return t.Local().YearDay() == time.Now().YearDay()
	})
}

func (s *savingsStats) existingPreviousWeekTotalSavings() bool {
	return s.existingTotalSavings(previousWeek, func(t *time.Time) bool {
		_, pw := t.Local().ISOWeek()
		_, w := time.Now().ISOWeek()
		return pw == w
	})
}

func (s *savingsStats) existingPreviousMonthTotalSavings() bool {
	return s.existingTotalSavings(previousMonth, func(t *time.Time) bool {
		return t.Local().Month() == time.Now().Month()
	})
}

func (s *savingsStats) existingPreviousYearTotalSavings() bool {
	return s.existingTotalSavings(previousYear, func(t *time.Time) bool {
		return t.Local().Year() == time.Now().Year()
	})
}

func (s *savingsStats) persistSavings(exists bool, timeline string, savingsCalculation func() float64) {

	if exists {
		log.Printf("Current %s saving figure was already set, skipping calculation\n", timeline)
		return
	}
	savings := savingsCalculation()

	log.Printf("Calculated %s savings %f", timeline, savings)
	if savings < 0.00001 {
		log.Printf("Calculated %s savings %f are negligible, we won't persit them in SSM", timeline, savings)
		return
	}
	param := fmt.Sprintf(SSMParameterNameTotalFormat, timeline)
	s.ps.putSSMParameter(param, fmt.Sprintf("%f", savings))
}

func (s *savingsStats) persistTotalHourlySavings(savings float64) {
	s.persistSavings(s.existingCurrentHourTotalSavings(), hourly, func() float64 {
		return savings
	})
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
	s.persistSavings(s.existingPreviousDayTotalSavings(), previousDay, s.calculatePreviousDayTotalSavings)
}

func (s *savingsStats) persistPreviousWeekTotalSavings() {
	s.persistSavings(s.existingPreviousWeekTotalSavings(), previousWeek, s.calculatePreviousWeekTotalSavings)
}

func (s *savingsStats) persistPreviousMonthTotalSavings() {
	s.persistSavings(s.existingPreviousMonthTotalSavings(), previousMonth, s.calculatePreviousMonthTotalSavings)
}

func (s *savingsStats) persistPreviousYearTotalSavings() {
	s.persistSavings(s.existingPreviousYearTotalSavings(), previousYear, s.calculatePreviousYearTotalSavings)
}

func (s *savingsStats) updateSavingsStats(savings float64) {
	s.persistTotalHourlySavings(savings)
	s.loadAllHistory()
	s.persistPreviousDayTotalSavings()
	s.persistPreviousWeekTotalSavings()
	s.persistPreviousMonthTotalSavings()
	s.persistPreviousYearTotalSavings()

	s.loadAllHistory() // load again after updating totals

	s.reportSavingsStats(s.reportFrequency,
		s.calculatePreviousDayTotalSavings(),
		s.calculateWeekSoFarTotalSavings(),
		s.calculatePreviousWeekTotalSavings(),
		s.calculateMonthSoFarTotalSavings(),
		s.calculatePreviousMonthTotalSavings(),
		s.calculateYearSoFarTotalSavings(),
		s.calculateAllTotalSavings())
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

func (s *savingsStats) formatSavings(frequency string,
	previousDay, currentWeek, previousWeek, currentMonth, previousMonth, currentYear, allTime float64) string {

	savings := fmt.Sprintf("- yesterday: $%.2f\n", previousDay)
	if frequency == "daily" {
		savings += fmt.Sprintf("- this week so far: $%.2f\n", currentWeek)
	}

	savings += fmt.Sprintf("- last week: $%.2f\n", previousWeek)

	if frequency != "monthly" {
		savings += fmt.Sprintf("- this month so far: $%.2f\n", currentMonth)
	}

	savings += fmt.Sprintf("- last month: $%.2f\n- this year so far: $%.2f\n- all time $%.2f\n",
		previousMonth, currentYear, allTime)

	return savings
}

func (s *savingsStats) formatSavingsMessage(frequency string,
	previousDay, currentWeek, previousWeek, currentMonth, previousMonth, currentYear, allTime float64) string {

	preamble := fmt.Sprintf(`
  Hi there,

  Here's your %s AutoSpotting Savings Report.

  Savings...`, frequency)

	ctas := `
  If you like these results it would be great if you could spread the word, even if just within your company by forwarding this email to your management or peers.

  I also like it a lot when people share good results with me and give a short testimonial or post them on social media.

  If you think there's room for more, I'd love to help you expand your AutoSpotting rollout (free of charge!) or in general with cost optimization work unrelated to AutoSpotting or also with ongoing FinOps services.

  I'd also love to hear from you for any questions, issues or feedback about AutoSpotting.
  You can reach out on Slack at https://bit.ly/CloudUtilSlack or by email at contact@autospotting.io.

  Cheers,
  Cristian - the author of AutoSpotting

  PS:
  - You may also want to check out the other tools from my CloudUtil portfolio: https://go.aws/3h7l7S9`

	return preamble + s.formatSavings(frequency,
		previousDay, currentWeek, previousWeek, currentMonth, previousMonth, currentYear, allTime) + ctas
}

func (s *savingsStats) reportSavingsStats(frequency string,
	previousDay, currentWeek, previousWeek, currentMonth, previousMonth, currentYear, allTime float64) {

	if !s.shouldReportSavingsStats() {
		return
	}

	log.Println("Sending Savings Report to SNS Topic:", s.topic)

	s.sns.Publish(
		&sns.PublishInput{
			Message: aws.String(s.formatSavingsMessage(frequency,
				previousDay, currentWeek, previousWeek, currentMonth, previousMonth, currentYear, allTime)),
			Subject:  aws.String("AutoSpotting savings report - " + time.Now().Format("Mon, 2 Jan 2006 15:04")),
			TopicArn: aws.String(s.topic),
		},
	)
	s.ps.putSSMParameter(SSMParameterNameLastReportName, time.Now().Format("Mon, 2 Jan 2006 15:04"))
}
