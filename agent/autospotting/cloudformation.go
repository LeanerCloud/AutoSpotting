package autospotting

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sns"
)

type cloudFormation struct {
	AWSConnections connections
}

// This data structure is used in order to generate a response to the
// CloudFormation operation, because CloudFormation always blocks waiting for
// such a response from the custom resource.
type cloudFormationCustomResourceResponse struct {
	Status             string
	PhysicalResourceID string
	StackID            string
	RequestID          string
	LogicalResourceID  string
	Data               map[string]interface{}
}

func (cfn *cloudFormation) processStackUpdate(
	e eventData,
	c contextData,
	cronTopic string) {

	status := "SUCCESS"

	logger.Println("Processing CloudFormation operation", e.RequestType)
	logger.Printf("Event: %v, Context: %v\n", e, c)

	// only handle the creation of the stack, all other operations are NOOPs
	if e.RequestType == "Create" {

		// subscribe to the topic that calls the function every 5 min
		err := cfn.connectLambdaToTopic(e.ServiceToken, cronTopic)

		if err != nil {
			logger.Println(err.Error())
			status = "FAILURE"
		}
	}

	cfn.provideCustomResourceResponse(e, c, status)

}

func (cfn *cloudFormation) connectLambdaToTopic(
	lambdaFunc string, topicARN string) error {

	var err error

	logger.Printf("Connecting lambda %v to topic %v\n", lambdaFunc, topicARN)

	if err = cfn.addLambdaInvokePermission(lambdaFunc, topicARN); err != nil {
		logger.Println(err.Error())
	}

	if err = cfn.subscribeLambdaToTopic(lambdaFunc, topicARN); err != nil {
		logger.Println(err.Error())
	}

	return err
}

func (cfn *cloudFormation) addLambdaInvokePermission(lambdaFunc string,
	topicARN string) error {

	logger.Printf("Adding invoke permissions for lambda %v to topic %v\n",
		lambdaFunc, topicARN)

	svc := cfn.AWSConnections.lambda

	statementID := strconv.Itoa(int(time.Now().UnixNano()))

	logger.Println("Adding invoke permission: ", lambdaFunc, topicARN)

	params := lambda.AddPermissionInput{
		Action:       aws.String("lambda:invokeFunction"),
		FunctionName: aws.String(lambdaFunc),
		Principal:    aws.String("sns.amazonaws.com"),
		StatementId:  aws.String(statementID),
		SourceArn:    aws.String(topicARN),
	}

	fmt.Printf("Function: '%s', statement: '%s', topic: '%s'",
		lambdaFunc, statementID, topicARN)

	fmt.Printf("Params: '%s'", svc)

	resp, err := svc.AddPermission(&params)

	if err != nil {
		logger.Println(err.Error())
	}

	// Pretty-print the response data.
	logger.Println(awsutil.StringValue(resp))
	return err
}

func (cfn *cloudFormation) subscribeLambdaToTopic(
	lambdaFunc string, topicARN string) error {

	logger.Println("Subscribing lambda", lambdaFunc, "to topic: ", topicARN)

	svc := cfn.AWSConnections.sns

	params := &sns.SubscribeInput{
		TopicArn: aws.String(topicARN),
		Protocol: aws.String("lambda"),
		Endpoint: aws.String(lambdaFunc),
	}

	resp, err := svc.Subscribe(params)

	if err != nil {
		logger.Println(err.Error())
	}

	// Pretty-print the response data.
	logger.Println(awsutil.StringValue(resp))

	return err
}

func (cfn *cloudFormation) provideCustomResourceResponse(
	e eventData, c contextData, status string) {

	// create response data
	var r cloudFormationCustomResourceResponse
	r.Status = status
	r.PhysicalResourceID = c.LogStreamName
	r.StackID = e.StackID
	r.RequestID = e.RequestID
	r.LogicalResourceID = e.LogicalResourceID
	r.Data = map[string]interface{}{"foo": "bar"}

	logger.Println(r)

	jsonStr, err := json.Marshal(r)

	if err != nil {
		logger.Println("Failed to marshal PUT response", err.Error())
	}

	logger.Println("Response payload:", string(jsonStr))

	// prepare an HTTP request
	req, err := http.NewRequest("PUT", e.ResponseURL,
		strings.NewReader(string(jsonStr)))

	if err != nil {
		logger.Println("Failed to create PUT request", err.Error())
	}

	// prepare the S3 upload URL
	URL, err := url.Parse(e.ResponseURL)
	if err != nil {
		logger.Println(err)
	}

	// tweak the URL decoding of the URL path
	req.URL.Opaque = strings.Replace(URL.Path, ":", "%3A", -1)
	req.URL.Opaque = strings.Replace(req.URL.Opaque, "|", "%7C", -1)

	// set some headers
	req.Header.Set("content-length", strconv.Itoa(len(jsonStr)))

	// perform the actual PUT request
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		logger.Println("Failed to set CloudFormation state", err.Error())
	}

	defer resp.Body.Close()

	logger.Println("response Status:", resp.Status)
	logger.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	logger.Println("response Body:", string(body))

}
