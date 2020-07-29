// Copyright (c) 2016-2019 Cristian Măgherușan-Stanciu
// Licensed under the Open Software License version 3.0

package autospotting

import (
	"time"

	"github.com/robfig/cron/v3"
)

// insideSchedule returns true if the time given in the t parameter is matching
// the implified cronrab-like interval restricted to only hours and days of the
// week. Because the cron library be use only supports the local time, the
// crontab entry will have to be created accoringly. When executed in Lambda the
// runtime's local time will always be UTC, so users have to be made aware of
// this through the documentation.
func insideSchedule(t time.Time, crontab string, timezone string) (bool, error) {
	// Get the timezone, will cause an error if timezone is incorrect
	tz, err := time.LoadLocation(timezone)

	if err != nil {
		logger.Println(err)
		return false, err
	}

	// Create a new cron job runner using the location details and a custom parser
	c := cron.New(cron.WithLocation(tz), cron.WithParser(cron.NewParser(cron.Hour|cron.Dow)))
	// Schedule an empty job based on out crontab
	entry, err := c.AddFunc(crontab, nil)

	if err != nil {
		logger.Println(err)
		return false, err
	}

	// Grab the schedule for the job created above to use for calculations below.
	sched := c.Entry(entry).Schedule

	// When inside the cron interval, the next event from exactly an hour ago and the
	// next event from now are exactly one hour apart
	prev := sched.Next(t.In(tz).Add(-1 * time.Hour))
	next := sched.Next(t.In(tz))

	if next == prev.Add(1*time.Hour) {
		return true, nil
	}
	return false, nil
}

// returns true if the schedule is "on" and we're inside the interval also
// returns true if the schedule is "off" and we're outside the interval returns
// false in case of cron parsing error and other schedule parameter combinations
func cronRunAction(t time.Time, crontab string, timezone string, scheduleType string) bool {
	inside, err := insideSchedule(t, crontab, timezone)
	debug.Println("Inside schedule for", crontab, ":", inside)

	if err != nil {
		return false
	}

	if (inside && scheduleType == "on") || (!inside && scheduleType == "off") {
		return true
	}

	return false
}
