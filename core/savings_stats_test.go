package autospotting

import (
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func Test_savingsStats_formatSavings(t *testing.T) {

	type args struct {
		last24Hours  float64
		last7Days    float64
		last30Days   float64
		last12Months float64
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "daily report",
			args: args{
				last24Hours:  1,
				last7Days:    3,
				last30Days:   5,
				last12Months: 6,
			},
			want: `- last 24 hours: $1.00
- last 7 days: $3.00
- last 30 days: $5.00
- last 12 months: $6.00
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{}
			if got := s.formatSavings(tt.args.last24Hours,
				tt.args.last7Days, tt.args.last30Days,
				tt.args.last12Months); got != tt.want {
				t.Errorf("savingsStats.formatSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateLast24HoursTotalSavings(t *testing.T) {

	today1 := time.Now().Add(-1 * time.Hour)
	today2 := time.Now().Add(-2 * time.Hour)
	yesterday1 := today1.Add(-24 * time.Hour)
	yesterday2 := today2.Add(-24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar today and 0.3 dollar yesterday",
			history: &savingsHistory{
				hourlyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &today1,
						Value:            aws.String("0.5"),
					},
					{
						LastModifiedDate: &today2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &yesterday1,
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &yesterday2,
						Value:            aws.String("0.2"),
					},
				},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{

				history: tt.history,
			}
			if got := s.calculateLast24HoursTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateLast24HoursTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateLast7DaysTotalSavings(t *testing.T) {
	day1 := time.Now().Add(-1 * time.Hour)
	day2 := time.Now().Add(-25 * time.Hour)
	lastWeekDay1 := day1.Add(-24 * 7 * time.Hour)
	lastWeekDay2 := day2.Add(-24 * 7 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this week and 0.3 dollar last week hourly",
			history: &savingsHistory{
				hourlyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &day1,
						Value:            aws.String("0.5"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastWeekDay1,
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastWeekDay2,
						Value:            aws.String("0.2"),
					},
				},
			},
			want: 0,
		},
		{
			name: "2 dollar this week and 0.3 dollar last week daily",
			history: &savingsHistory{
				dailyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &day1,
						Value:            aws.String("0.5"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastWeekDay1,
						Value:            aws.String("0.1"), //ignored in the calculation
					},
					{
						LastModifiedDate: &lastWeekDay2,
						Value:            aws.String("0.2"), //ignored in the calculation
					},
				},
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculateLast7DaysTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateLast7DaysTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateLast30DaysTotalSavings(t *testing.T) {
	// Use a fixed time for testing.
	now := time.Date(2023, 7, 3, 0, 0, 0, 0, time.UTC)

	day1 := now.Add(-1 * time.Hour)
	day2 := day1.Add(-25 * time.Hour)
	lastMonthDay1 := day1.Add(-30 * 24 * time.Hour)
	lastMonthDay2 := day2.Add(-30 * 24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly",
			history: &savingsHistory{
				hourlyHistory: []*ssm.ParameterHistory{ // all these will be ignored in the calculation
					{
						LastModifiedDate: &day1,
						Value:            aws.String("0.5"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastMonthDay1,
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastMonthDay2,
						Value:            aws.String("0.2"),
					},
				},
			},
			want: 0,
		},
		{
			name: "2.5 dollar this month and 0.4 dollar last month daily",
			history: &savingsHistory{
				dailyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &day1,
						Value:            aws.String("1"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastMonthDay1,
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastMonthDay2,
						Value:            aws.String("0.3"), // more than 30 days ago and would be ignored
					},
				},
			},
			want: 2.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			// Use the fixed time inside the time filter.
			if got := s.calculateSavingsFromHistory(s.history.dailyHistory, func(t *time.Time) bool {
				ret := t.Local().After(now.AddDate(0, -1, 0))
				debug.Printf("Checking if timestamp %v is from last 30 days: %v\n", t.Local(), ret)
				return ret
			}); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateLast30DaysTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateLast12MonthsTotalSavings(t *testing.T) {
	day1 := time.Now().Add(-1 * time.Hour)
	day2 := day1.Add(-40 * 24 * time.Hour)
	lastYearDay1 := day1.Add(-150 * 24 * time.Hour)
	lastYearDay2 := day2.Add(-365 * 24 * time.Hour) //older than a year
	yesterday := day1.Add(-24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly", //ignored in the calculation
			history: &savingsHistory{
				hourlyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &day1,
						Value:            aws.String("0.5"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastYearDay1,
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastYearDay2,
						Value:            aws.String("0.2"),
					},
				},
			},
			want: 0,
		},
		{
			name: "2.5 dollar this month and 0.4 dollar last month daily", //ignored in the calculation
			history: &savingsHistory{
				dailyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &day1,
						Value:            aws.String("1"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastYearDay1,
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastYearDay2,
						Value:            aws.String("0.3"),
					},
				},
			},
			want: 0,
		},
		{
			name: "2.5 dollar this year and 0.4 dollar last year monthly",
			history: &savingsHistory{
				dailyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &yesterday,
						Value:            aws.String("3"), //ignored in the calculation
					},
				},
				monthlyHistory: []*ssm.ParameterHistory{
					{
						LastModifiedDate: &day1,
						Value:            aws.String("1"),
					},
					{
						LastModifiedDate: &day2,
						Value:            aws.String("1.5"),
					},
					{
						LastModifiedDate: &lastYearDay1,
						Value:            aws.String("0.2"),
					},
					{
						LastModifiedDate: &lastYearDay2,
						Value:            aws.String("0.3"), //ignored in the calculation
					},
				},
			},
			want: 2.7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculateLast12MonthsTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateLast12MonthsTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}
