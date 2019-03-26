import boto3
import botocore
import json
import sys
import traceback

from botocore.vendored import requests

SUCCESS = "SUCCESS"
FAILED = "FAILED"
STACK_NAME = 'AutoSpottingRegionalResources'
TEMPLATE_URL = 'https://s3.amazonaws.com/cloudprowess/nightly/regional_template.yaml'


def handle_delete(event):
    ec2 = boto3.client('ec2')
    wait_for_deletion = False
    for region in ec2.describe_regions()['Regions']:
        client = boto3.client('cloudformation', region['RegionName'])
        # try except to make sure stack exists before deletion
        try:
            client.describe_stacks(
                StackName=STACK_NAME
            )
            client.delete_stack(
                StackName=STACK_NAME
            )
            wait_for_deletion = True
        except botocore.exceptions.ClientError:
            pass

    # Wait only for last otherwise stack deletion will take forever
    if wait_for_deletion == True:
        waiter = client.get_waiter('stack_delete_complete')
        waiter.wait(
            StackName=STACK_NAME
        )


def handle_create(event):
    ec2 = boto3.client('ec2')
    lambda_arn = event['ResourceProperties']['LambdaARN']

    for region in ec2.describe_regions()['Regions']:
        client = boto3.client('cloudformation', region['RegionName'])
        # try except to make sure stack does not exist before creation
        try:
            client.describe_stacks(
                StackName=STACK_NAME
            )
        except botocore.exceptions.ClientError:
            client.create_stack(
                StackName=STACK_NAME,
                TemplateURL=TEMPLATE_URL,
                Parameters=[
                    {
                        'ParameterKey': 'AutoSpottingLambdaARN',
                        'ParameterValue': lambda_arn,
                    },
                ],
            )


def handler(event, context):
    try:
        if event['RequestType'] == 'Delete':
            handle_delete(event)

        if event['RequestType'] == 'Create':
            handle_create(event)
        send(event, context, SUCCESS, {})
    except:
        traceback.print_exc()
        print("Unexpected error:", sys.exc_info()[0])
        send(event, context, FAILED, {})


def send(event, context, responseStatus, responseData, physicalResourceId=None, noEcho=False):
    responseUrl = event['ResponseURL']

    print(responseUrl)

    responseBody = {}
    responseBody['Status'] = responseStatus
    responseBody['Reason'] = 'See the details in CloudWatch Log Stream: ' + context.log_stream_name
    responseBody['PhysicalResourceId'] = physicalResourceId or context.log_stream_name
    responseBody['StackId'] = event['StackId']
    responseBody['RequestId'] = event['RequestId']
    responseBody['LogicalResourceId'] = event['LogicalResourceId']
    responseBody['NoEcho'] = noEcho
    responseBody['Data'] = responseData

    json_responseBody = json.dumps(responseBody)

    print("Response body:\n" + json_responseBody)

    headers = {
        'content-type': '',
        'content-length': str(len(json_responseBody))
    }

    try:
        response = requests.put(responseUrl,
                                data=json_responseBody,
                                headers=headers)
        print("Status code: " + response.reason)
    except Exception as e:
        print("send(..) failed executing requests.put(..): " + str(e))
