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
)

// MeteringSSMParameterName stores the name of the SSM parameter that stores the success status of the latest metering call
const (
	SSMParameterNameTotalFormat    = "/autospotting/savings/%s/total"
	SSMParameterNameByASGFormat    = "/autospotting/savings/%s/%s/%s" // time_interval/region/asg_name, eg hourly/us-east1-1/my_asg
	SSMParameterNameLastReportName = "/autospotting/savings/report_timestamp"

	hourly = "hourly"

	daily   = "daily"
	monthly = "monthly"
)

type savingsStats struct {
	ps              parameterStore
	sns             snsiface.SNSAPI
	topic           string
	reportFrequency string
	history         *savingsHistory
}

type savingsHistory struct {
	hourlyHistory, dailyHistory, monthlyHistory []*ssm.ParameterHistory
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
		dailyHistory:   s.loadHistory(daily),
		monthlyHistory: s.loadHistory(monthly),
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

func (s *savingsStats) calculateLast24HoursTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.hourlyHistory, func(t *time.Time) bool {
		ret := t.Local().After(time.Now().AddDate(0, 0, -1))
		debug.Printf("Checking if timestamp %v is from the last 24h: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateLast7DaysTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
		ret := t.Local().After(time.Now().AddDate(0, 0, -7))
		debug.Printf("Checking if timestamp %v is from last 7 days: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateLast30DaysTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
		ret := t.Local().After(time.Now().AddDate(0, -1, 0))
		debug.Printf("Checking if timestamp %v is from last 30 days: %v\n", t.Local(), ret)
		return ret
	})
}

func (s *savingsStats) calculateLast12MonthsTotalSavings() float64 {
	return s.calculateSavingsFromHistory(s.history.monthlyHistory, func(t *time.Time) bool {
		ret := t.Local().After(time.Now().AddDate(-1, 0, 0))
		debug.Printf("Checking if timestamp %v is from last 12 months: %v\n", t.Local(), ret)
		return ret
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

func (s *savingsStats) updatedHourlyTotalSavings() bool {
	return s.existingTotalSavings(hourly, func(t *time.Time) bool {
		return t.Local().Hour() == time.Now().Hour()
	})
}

func (s *savingsStats) updatedDailyTotalSavings() bool {
	return s.existingTotalSavings(daily, func(t *time.Time) bool {
		return t.Local().YearDay() == time.Now().YearDay()
	})
}

func (s *savingsStats) updatedMonthlyTotalSavings() bool {
	return s.existingTotalSavings(monthly, func(t *time.Time) bool {
		return t.Local().Month() == time.Now().Month()
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
	s.persistSavings(s.updatedHourlyTotalSavings(), hourly, func() float64 {
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
	s.persistSavings(s.updatedDailyTotalSavings(), daily, s.calculateLast24HoursTotalSavings)
}

func (s *savingsStats) persistPreviousMonthTotalSavings() {
	s.persistSavings(s.updatedMonthlyTotalSavings(), monthly, s.calculateLast30DaysTotalSavings)
}

func (s *savingsStats) updateSavingsStats(savings float64) {
	s.persistTotalHourlySavings(savings)
	s.loadAllHistory()
	s.persistPreviousDayTotalSavings()
	s.persistPreviousMonthTotalSavings()
	s.loadAllHistory() // load again after updating totals

	s.reportSavingsStats(s.reportFrequency,
		s.calculateLast24HoursTotalSavings(),
		s.calculateLast7DaysTotalSavings(),
		s.calculateLast30DaysTotalSavings(),
		s.calculateLast12MonthsTotalSavings())
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

func (s *savingsStats) formatSavings(
	last24hours, last7days, last30days, last12months float64) string {

	savings := fmt.Sprintf("- last 24 hours: $%.2f\n", last24hours)
	savings += fmt.Sprintf("- last 7 days: $%.2f\n", last7days)
	savings += fmt.Sprintf("- last 30 days: $%.2f\n", last30days)
	savings += fmt.Sprintf("- last 12 months: $%.2f\n", last12months)

	return savings
}

func (s *savingsStats) formatSavingsMessage(frequency string,
	last24hours, last7days, last30days, last12months float64) string {

	preamble := fmt.Sprintf(`
  Hi there,

  Here's your %s AutoSpotting Savings Report.

  Savings...\n`, frequency)

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

	return preamble + s.formatSavings(
		last24hours, last7days, last30days, last12months) + ctas
}

func (s *savingsStats) reportSavingsStats(frequency string,
	last24hours, last7days, last30days, last12months float64) {

	if !s.shouldReportSavingsStats() {
		return
	}

	log.Println("Sending Savings Report to SNS Topic:", s.topic)

	s.sns.Publish(
		&sns.PublishInput{
			Message: aws.String(s.formatSavingsMessage(frequency,
				last24hours, last7days, last30days, last12months)),
			Subject:  aws.String("AutoSpotting savings report - " + time.Now().Format("Mon, 2 Jan 2006 15:04")),
			TopicArn: aws.String(s.topic),
		},
	)
	s.ps.putSSMParameter(SSMParameterNameLastReportName, time.Now().Format("Mon, 2 Jan 2006 15:04"))
}
