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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/sns"
)

type cloudFormation struct {
	AWSConnections connections
}

// This data structure is used in order to generate a response to the CloudFormation operation,
// because CloudFormation always blocks waiting for such a response from the custom resource.
type cloudFormationCustomResourceResponse struct {
	status             string
	physicalResourceID string
	stackID            string
	requestID          string
	logicalResourceID  string
	data               map[string]interface{}
}

func (cfn *cloudFormation) processStackUpdate(e eventData, c contextData, cronTopic string) {
	status := "SUCCESS"

	fmt.Println("Processing CloudFormation operation", e.RequestType)
	fmt.Println("Event: %v, Context: %v", e, c)

	// only handle the creation of the stack, all other operations are NOOPs
	if e.RequestType == "Create" {

		// subscribe to the topic that calls the function every 5 min
		err := cfn.connectLambdaToTopic(e.ServiceToken, cronTopic)

		if err != nil {
			fmt.Println(err.Error())
			status = "FAILURE"
		}
	}

	cfn.provideCustomResourceResponse(e, c, status)

}

func (cfn *cloudFormation) connectLambdaToTopic(lambdaFunc string, topicARN string) error {
	fmt.Printf("Connecting lambda %v to topic %v\n", lambdaFunc, topicARN)
	err := cfn.addLambdaInvokePermission(lambdaFunc, topicARN)
	if err != nil {
		fmt.Println(err.Error())
	}

	err = cfn.subscribeLambdaToTopic(lambdaFunc, topicARN)
	if err != nil {
		fmt.Println(err.Error())
	}

	return err
}
func (cfn *cloudFormation) addLambdaInvokePermission(lambdaFunc string, topicARN string) error {

	fmt.Print("Adding invoke permissions for lambda %v to topic %v\n", lambdaFunc, topicARN)

	// re-implements this kind of command using API calls:
	// aws lambda add-permission --function-name lambda-LambdaFunction-1KVSRHYIWUSBO
	// --action 'lambda:invokeFunction' --principal sns.amazonaws.com
	// --statement-id 2 --source-arn arn:aws:sns:eu-west-1:540659244915:Notifications-EU

	svc := cfn.AWSConnections.lambda

	statementID := strconv.Itoa(int(time.Now().UnixNano()))
	fmt.Println("Adding invoke permission: ", lambdaFunc, topicARN)

	params := &lambda.AddPermissionInput{
		Action:       aws.String("lambda:invokeFunction"), // Required
		FunctionName: aws.String(lambdaFunc),              // Required
		Principal:    aws.String("sns.amazonaws.com"),     // Required
		StatementId:  aws.String(statementID),             // Required
		SourceArn:    aws.String(topicARN),
	}

	_ = "breakpoint"

	resp, err := svc.AddPermission(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS error with Code, Message, and original error (if any)
			fmt.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				fmt.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
			}
		} else {
			// This case should never be hit, the SDK should always return an
			// error which satisfies the awserr.Error interface.
			fmt.Println(err.Error())
		}
	}

	// Pretty-print the response data.
	fmt.Println(awsutil.StringValue(resp))
	return err
}

func (cfn *cloudFormation) subscribeLambdaToTopic(lambdaFunc string, topicARN string) error {

	fmt.Println("Subscribing lambda", lambdaFunc, "to topic: ", topicARN)

	svc := cfn.AWSConnections.sns

	params := &sns.SubscribeInput{
		TopicArn: aws.String(topicARN),
		Protocol: aws.String("lambda"),
		Endpoint: aws.String(lambdaFunc),
	}

	resp, err := svc.Subscribe(params)

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Generic AWS error with Code, Message, and original error (if any)
			fmt.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
			if reqErr, ok := err.(awserr.RequestFailure); ok {
				// A service error occurred
				fmt.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
			}
		} else {
			// This case should never be hit, the SDK should always return an
			// error which satisfies the awserr.Error interface.
			fmt.Println(err.Error())
		}
	}

	// Pretty-print the response data.
	fmt.Println(awsutil.StringValue(resp))

	return err
}

func (cfn *cloudFormation) provideCustomResourceResponse(e eventData, c contextData, status string) {

	// create response data
	var r cloudFormationCustomResourceResponse
	r.status = status
	r.physicalResourceID = c.LogStreamName
	r.stackID = e.StackID
	r.requestID = e.RequestID
	r.logicalResourceID = e.LogicalResourceID
	r.data = map[string]interface{}{"foo": "bar"}

	jsonStr, _ := json.Marshal(r)

	fmt.Println("Response payload:", string(jsonStr))

	// prepare an HTTP request
	req, err := http.NewRequest("PUT", e.ResponseURL, strings.NewReader(string(jsonStr)))

	if err != nil {
		fmt.Println("Failed to create PUT request", err.Error())
	}

	// prepare the S3 upload URL
	URL, err := url.Parse(e.ResponseURL)
	if err != nil {
		fmt.Println(err)
	}

	// tweak the URL decoding of the URL path
	req.URL.Opaque = strings.Replace(URL.Path, ":", "%3A", -1)
	req.URL.Opaque = strings.Replace(req.URL.Opaque, "|", "%7C", -1)

	// set some headers
	req.Header.Set("content-length", strconv.Itoa(len(jsonStr)))

	// fire the actual PUT request
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		fmt.Println("Failed to set CloudFormation state", err.Error())
	}
	defer resp.Body.Close()

	fmt.Println("response Status:", resp.Status)
	fmt.Println("response Headers:", resp.Header)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("response Body:", string(body))

}
