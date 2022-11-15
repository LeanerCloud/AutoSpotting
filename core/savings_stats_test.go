package autospotting

import (
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/jinzhu/now"
)

func Test_savingsStats_formatSavings(t *testing.T) {

	type args struct {
		frequency     string
		previousDay   float64
		currentWeek   float64
		previousWeek  float64
		currentMonth  float64
		previousMonth float64
		currentYear   float64
		allTime       float64
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "daily report",
			args: args{
				frequency:     "daily",
				previousDay:   1,
				currentWeek:   2,
				previousWeek:  3,
				currentMonth:  4,
				previousMonth: 5,
				currentYear:   6,
				allTime:       7,
			},
			want: `- yesterday: $1.00
- this week so far: $2.00
- last week: $3.00
- this month so far: $4.00
- last month: $5.00
- this year so far: $6.00
- all time $7.00
`,
		},

		{
			name: "weekly report",
			args: args{
				frequency:     "weekly",
				previousDay:   1,
				currentWeek:   2,
				previousWeek:  3,
				currentMonth:  4,
				previousMonth: 5,
				currentYear:   6,
				allTime:       7,
			},
			want: `- yesterday: $1.00
- last week: $3.00
- this month so far: $4.00
- last month: $5.00
- this year so far: $6.00
- all time $7.00
`,
		},

		{
			name: "monthly report",
			args: args{
				frequency:     "monthly",
				previousDay:   1,
				currentWeek:   2,
				previousWeek:  3,
				currentMonth:  4,
				previousMonth: 5,
				currentYear:   6,
				allTime:       7,
			},
			want: `- yesterday: $1.00
- last week: $3.00
- last month: $5.00
- this year so far: $6.00
- all time $7.00
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{}
			if got := s.formatSavings(tt.args.frequency, tt.args.previousDay, tt.args.currentWeek,
				tt.args.previousWeek, tt.args.currentMonth, tt.args.previousMonth,
				tt.args.currentYear, tt.args.allTime); got != tt.want {
				t.Errorf("savingsStats.formatSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateDaySoFarTotalSavings(t *testing.T) {

	today1 := now.BeginningOfDay().Add(time.Hour)
	today2 := now.BeginningOfDay().Add(2 * time.Hour)
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
			if got := s.calculateDaySoFarTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateDaySoFarTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculatePreviousDayTotalSavings(t *testing.T) {
	today1 := now.BeginningOfDay().Add(time.Hour)
	today2 := now.BeginningOfDay().Add(2 * time.Hour)
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
			want: 0.3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{

				history: tt.history,
			}
			if got := s.calculatePreviousDayTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculatePreviousDayTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateWeekSoFarTotalSavings(t *testing.T) {
	day1 := now.BeginningOfWeek().Add(time.Hour)
	day2 := now.BeginningOfWeek().Add(25 * time.Hour)
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
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastWeekDay2,
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
			if got := s.calculateWeekSoFarTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateWeekSoFarTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculatePreviousWeekTotalSavings(t *testing.T) {
	day1 := now.BeginningOfWeek().Add(time.Hour)
	day2 := now.BeginningOfWeek().Add(25 * time.Hour)
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
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastWeekDay2,
						Value:            aws.String("0.2"),
					},
				},
			},
			want: 0.3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculatePreviousWeekTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculatePreviousWeekTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateMonthSoFarTotalSavings(t *testing.T) {
	day1 := now.BeginningOfMonth().Add(time.Hour)
	day2 := now.BeginningOfMonth().Add(25 * time.Hour)
	lastMonthDay1 := day1.Add(-20 * 24 * time.Hour)
	lastMonthDay2 := day2.Add(-20 * 24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly",
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
						Value:            aws.String("0.3"),
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
			if got := s.calculateMonthSoFarTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateMonthSoFarTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculatePreviousMonthTotalSavings(t *testing.T) {
	day1 := now.BeginningOfMonth().Add(time.Hour)
	day2 := now.BeginningOfMonth().Add(25 * time.Hour)
	lastMonthDay1 := day1.Add(-20 * 24 * time.Hour)
	lastMonthDay2 := day2.Add(-20 * 24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly",
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
						Value:            aws.String("0.3"),
					},
				},
			},
			want: 0.4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculatePreviousMonthTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculatePreviousMonthTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateYearSoFarTotalSavings(t *testing.T) {
	day1 := now.BeginningOfYear().Add(time.Hour)
	day2 := now.BeginningOfYear().Add(40 * 24 * time.Hour)
	lastYearDay1 := day1.Add(-150 * 24 * time.Hour)
	lastYearDay2 := day2.Add(-120 * 24 * time.Hour)
	yesterday := now.BeginningOfDay().Add(24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly",
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
						Value:            aws.String("3"),
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
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastYearDay2,
						Value:            aws.String("0.3"),
					},
				},
			},
			want: 5.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculateYearSoFarTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateYearSoFarTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculatePreviousYearTotalSavings(t *testing.T) {
	day1 := now.BeginningOfYear().Add(time.Hour)
	day2 := now.BeginningOfYear().Add(40 * 24 * time.Hour)
	lastYearDay1 := day1.Add(-150 * 24 * time.Hour)
	lastYearDay2 := day2.Add(-120 * 24 * time.Hour)
	yesterday := now.BeginningOfDay().Add(24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly",
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
						Value:            aws.String("3"),
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
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastYearDay2,
						Value:            aws.String("0.3"),
					},
				},
			},
			want: 0.4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculatePreviousYearTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculatePreviousYearTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_savingsStats_calculateAllTotalSavings(t *testing.T) {
	day1 := now.BeginningOfYear().Add(time.Hour)
	day2 := now.BeginningOfYear().Add(40 * 24 * time.Hour)
	lastYearDay1 := day1.Add(-150 * 24 * time.Hour)
	lastYearDay2 := day2.Add(-120 * 24 * time.Hour)
	yesterday := now.BeginningOfDay().Add(24 * time.Hour)

	tests := []struct {
		name    string
		history *savingsHistory
		want    float64
	}{
		{
			name: "2 dollar this month and 0.3 dollar last month hourly",
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
						Value:            aws.String("3"),
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
						Value:            aws.String("0.1"),
					},
					{
						LastModifiedDate: &lastYearDay2,
						Value:            aws.String("0.3"),
					},
				},
			},
			want: 5.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &savingsStats{
				history: tt.history,
			}
			if got := s.calculateAllTotalSavings(); math.Abs(got-tt.want) > 0.00000000001 {
				t.Errorf("savingsStats.calculateAllTotalSavings() = %v, want %v", got, tt.want)
			}
		})
	}
}
