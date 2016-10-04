package autospotting

import "sync"

// Run starts processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func Run(instancesFile string) {

	initLogger()

	var jsonInst jsonInstances

	logger.Println("Loading on-demand instance pricing information")
	jsonInst.loadFromFile(instancesFile)

	processAllRegions(&jsonInst)

}

func processAllRegions(instData *jsonInstances) {
	// for each region in parallel
	// for each of the ASGs tagged with 'spot-enabled=true'

	var wg sync.WaitGroup

	regions, err := getRegions()

	if err != nil {
		logger.Println(err.Error())
		return
	}
	for _, r := range regions {
		wg.Add(1)
		r := region{name: r}
		go func() {
			r.processRegion(instData)
			wg.Done()
		}()
	}
	wg.Wait()

}
