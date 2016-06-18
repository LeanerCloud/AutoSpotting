package autospotting

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// LambdaEventFromFiles is a data structure that once fully populated
// stores the input data coming from the user.
//
// The program assumes the existence of two files somewhere in
// the filesystem.
//
// They need to be passed as command line arguments, and
// The main function will populate EventFile and ContextFile
// after parsing the command line arguments.
//
// Later they are parsed into the eventData and contextData
// data structures, which are used as the only user-provided
// input to the program. All the rest is scanned from various
// AWS API calls, or from other sources.
type LambdaEventFromFiles struct {
	EventFile   string
	ContextFile string
	event       eventData
	context     contextData
	eventJSON   []byte
	contextJSON []byte
}

// This is a data structure we use for parsing the JSON we get in the event variable passed while running the lambda function.
// most fields are for the CloudFormation stack operations, which call the
// lambda function directly, except for the last one (Records), which is
// only used when calling the lambda function through SNS
type eventData struct {
	RequestType        string
	ServiceToken       string
	ResponseURL        string
	StackID            string
	RequestID          string
	LogicalResourceID  string
	ResourceType       string
	ResourceProperties resourceProperties

	// This field is the only one used for the SNS
	// messages we use for triggering the lambda function
	Records []snsEvent `json:"Records"`
}

type contextData struct {
	AWSRequestID       string `json:"awsRequestId"`
	InvokeID           string `json:"invokeid"`
	LogGroupName       string `json:"logGroupName"`
	LogStreamName      string `json:"logStreamName"`
	FunctionName       string `json:"functionName"`
	MemoryLimitInMB    string `json:"memoryLimitInMB"`
	FunctionVersion    string `json:"functionVersion"`
	InvokedFunctionArn string `json:"invokedFunctionArn"`
}

// Arguments passed to the lambda function by our custom resource which is
// executed whenever the CloudFormation stack is changed.
type resourceProperties struct {
	SNSTopic          string
	ServiceToken      string
	AutoScalingGroup  string
	InstanceGraceTime int `json:",string"`
}

// This data structure is populated if the lambda function is called using the SNS topic
type snsEvent struct {
	EventSource          string          `json:"EventSource"`
	EventVersion         string          `json:"EventVersion"`
	EventSubscriptionArn string          `json:"EventSubscriptionArn"`
	SNS                  snsNotification `json:"Sns"`
}

type snsNotification struct {
	NotificationType string `json:"Type"`
	MessageID        string `json:"MessageId"`
	TopicARN         string `json:"TopicArn"`
	Subject          string `json:"Subject"`

	// The message field often contains JSON content which then needs to be parsed and decoded further
	Message string `json:"Message"`

	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertUrl"`
	UnsubscribeURL   string `json:"UnsubscribeUrl"`
}

// HandleEvent is reading the event information and once it is parsed,
// it takes action by either running the CloudFormation handler,
// or start processing all AWS regions looking for AutoScaling groups
// enabled and taking action by replacing more pricy on-demand instances
// with compatible and cheaper spot instances.
func (e *LambdaEventFromFiles) HandleEvent(cronTopic, instancesURL string) {

	initLogger()

	e.readEventData()

	// logger.Println("Event: ", e.event)
	if e.event.RequestType == "" && e.event.Records[0].SNS.Message != "" {
		logger.Println("Detected execution triggered by an SNS Topic Notification")

		var ir instanceReplacement

		// TODO: we could cache this data locally for a while, like for a few days
		var jsonInst jsonInstances

		logger.Println("Loading on-demand instance pricing information")
		jsonInst.loadFromURL(instancesURL)

		//logger.Println(jsonInst)

		ir.processAllRegions(&jsonInst)

	} else {
		logger.Println("Detected a CloudFormation operation:", e.event.RequestType)
		logger.Println(e.context)
		var cfn cloudFormation
		cfn.processStackUpdate(e.event, e.context, cronTopic)

	}
}

func (e *LambdaEventFromFiles) readEventData() {

	eventFileContent, err := ioutil.ReadFile(e.EventFile)

	if err != nil {
		logger.Println(err.Error())
		os.Exit(1)
	}

	contextFileContent, err := ioutil.ReadFile(e.ContextFile)

	if err != nil {
		logger.Println(err.Error())
		os.Exit(1)
	}

	err = e.decodeEventData(eventFileContent, contextFileContent)

	if err != nil {
		logger.Println("Couldn't decode input data: ", err.Error())
		os.Exit(1)
	}
}

func (e *LambdaEventFromFiles) decodeEventData(eventJSON, contextJSON []byte) error {

	logger.Println("Got event: ", string(eventJSON))
	logger.Println("Got context: ", string(contextJSON))

	err := json.Unmarshal(eventJSON, &e.event)

	// logger.Println(e.event)

	if err != nil {
		logger.Println(err.Error())
		return err
	}

	err = json.Unmarshal(contextJSON, &e.context)

	if err != nil {
		logger.Println(err.Error())
		return err
	}
	// logger.Println(e.context)
	return nil
}
