package main

import (
	"fmt"
	"os"
	"time"
)

const dateFormat = "02-Jan-2006"

func main() {
	flavor := os.Getenv("FLAVOR")

	if flavor == "nightly" {
		// a month from now
		fmt.Println(time.Now().AddDate(0, 1, 0).Format(dateFormat))
	} else {
		// 100 years from now
		fmt.Println(time.Now().AddDate(100, 0, 0).Format(dateFormat))
	}
}
