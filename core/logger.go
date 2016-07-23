package autospotting

import (
	"log"
	"os"
)

var logger *log.Logger

func initLogger() {

	logger = log.New(os.Stdout, "", log.Lshortfile)

}
