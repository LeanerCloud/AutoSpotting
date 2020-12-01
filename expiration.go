package main

import "time"

const dateFormat = "02-Jan-2006"

func isExpired(date string) bool {
	exp, _ := time.Parse(dateFormat, date)
	return time.Now().After(exp)
}
