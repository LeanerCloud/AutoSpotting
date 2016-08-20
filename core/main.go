package autospotting

// Run starts processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances with
// compatible and cheaper spot instances.
func Run(instancesURL string) {

	initLogger()

	var ir instanceReplacement

	// TODO: we could cache this data locally for a while, like for a few days
	var jsonInst jsonInstances

	logger.Println("Loading on-demand instance pricing information")
	jsonInst.loadFromURL(instancesURL)

	// logger.Println(spew.Sdump(jsonInst))

	ir.processAllRegions(&jsonInst)

}
