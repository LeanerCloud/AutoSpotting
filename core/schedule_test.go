// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func Test_insideSchedule(t *testing.T) {

	tests := []struct {
		name     string
		t        time.Time
		timezone string
		crontab  string
		want     bool
		wantErr  error
	}{
		{
			name:     "All the time",
			crontab:  "* *",
			t:        time.Date(2019, time.May, 9, 10, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     true,
			wantErr:  nil,
		},
		{
			name:     "Inside business week",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 10, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     true,
			wantErr:  nil,
		},
		{
			name:     "Inside business week, before interval start",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 4, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  nil,
		},
		{
			name:     "Inside business week, after interval end",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 21, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  nil,
		},

		{
			name:     "Inside business week, One minute before interval start",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 8, 59, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  nil,
		},
		{
			name:     "Inside business week, One minute after interval start",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 9, 1, 0, 0, time.UTC),
			timezone: "UTC",
			want:     true,
			wantErr:  nil,
		},
		{
			name:     "Inside business week, One minute before interval end",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 17, 59, 0, 0, time.UTC),
			timezone: "UTC",
			want:     true,
			wantErr:  nil,
		},
		{
			name:     "Inside business week, One minute after interval end",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 18, 1, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  nil,
		},
		{
			name:     "During the weekend",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 11, 18, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  nil,
		},
		{
			name:     "During the weekend, but incorrect crontab",
			crontab:  "9- 1-5",
			t:        time.Date(2019, time.May, 11, 18, 0, 0, 0, time.UTC),
			timezone: "UTC",
			want:     false,
			wantErr:  errors.New("invalid syntax"),
		},
		{
			name:     "Inside business week, inside in timezone, inside in UTC",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 9, 1, 0, 0, time.UTC),
			timezone: "Europe/London",
			want:     true,
			wantErr:  nil,
		},
		{
			name:     "Inside business week, inside in timezone, outside in UTC",
			crontab:  "9-18 1-5",
			t:        time.Date(2019, time.May, 9, 8, 1, 0, 0, time.UTC),
			timezone: "Europe/London",
			want:     true,
			wantErr:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := insideSchedule(tt.t, tt.crontab, tt.timezone); got != tt.want ||
				// the err is checked for matching wantErr, doesn't need to be identical
				!(err == tt.wantErr || strings.Contains(err.Error(), tt.wantErr.Error())) {
				t.Errorf("insideSchedule() = %v, %v want %v, %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func Test_runAction(t *testing.T) {

	tests := []struct {
		name         string
		crontab      string
		t            time.Time
		timezone     string
		scheduleType string
		want         bool
	}{
		{
			name:         "On inside interval and currently in the interval",
			crontab:      "9-18 1-5",
			t:            time.Date(2019, time.May, 9, 10, 0, 0, 0, time.UTC),
			timezone:     "UTC",
			scheduleType: "on",
			want:         true,
		}, {
			name:         "On inside interval, but currently outside interval",
			crontab:      "9-18 1-5",
			t:            time.Date(2019, time.May, 9, 20, 0, 0, 0, time.UTC),
			timezone:     "UTC",
			scheduleType: "on",
			want:         false,
		},
		{
			name:         "On inside interval, but inside timezone, outside UTC",
			crontab:      "9-18 1-5",
			t:            time.Date(2019, time.May, 9, 8, 1, 0, 0, time.UTC),
			timezone:     "Europe/London",
			scheduleType: "on",
			want:         true,
		},
		{
			name:         "Off inside interval, and currently in the interval",
			crontab:      "9-18 1-5",
			t:            time.Date(2019, time.May, 9, 10, 0, 0, 0, time.UTC),
			timezone:     "UTC",
			scheduleType: "off",
			want:         false,
		}, {
			name:         "Off inside interval, and currently outside the interval",
			crontab:      "9-18 1-5",
			t:            time.Date(2019, time.May, 9, 20, 0, 0, 0, time.UTC),
			timezone:     "UTC",
			scheduleType: "off",
			want:         true,
		},
		{
			name:         "Off inside interval, but inside timezone, outside UTC",
			crontab:      "9-18 1-5",
			t:            time.Date(2019, time.May, 9, 8, 1, 0, 0, time.UTC),
			timezone:     "Europe/London",
			scheduleType: "off",
			want:         false,
		},
		{
			name:         "Incorrect cron rule",
			crontab:      "-18 1-5",
			t:            time.Date(2019, time.May, 9, 20, 0, 0, 0, time.UTC),
			timezone:     "UTC",
			scheduleType: "off",
			want:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cronRunAction(tt.t, tt.crontab, tt.timezone, tt.scheduleType); got != tt.want {
				t.Errorf("runAction() = %v, want %v", got, tt.want)
			}
		})
	}
}
