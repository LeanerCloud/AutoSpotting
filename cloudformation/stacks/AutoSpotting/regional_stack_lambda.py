import boto3
import botocore
import json
import sys
import traceback

from threading import Thread
from botocore.vendored import requests

SUCCESS = "SUCCESS"
FAILED = "FAILED"
STACK_NAME = 'AutoSpottingRegionalResources'
TEMPLATE_URL = 'https://s3.amazonaws.com/cloudprowess/nightly/regional_template.yaml'


def create_stack(region, lambda_arn):
    client = boto3.client('cloudformation', region)
    response = {}

    # delete stack if it already exists from a previous run
    try:
        response = client.delete_stack(StackName=STACK_NAME)
        print(response)
        waiter = client.get_waiter('stack_delete_complete')
        waiter.wait(
            StackName=STACK_NAME,
            WaiterConfig={'Delay': 5}
        )
    except botocore.exceptions.ClientError:
        pass

    try:
        response = client.create_stack(
            StackName=STACK_NAME,
            TemplateURL=TEMPLATE_URL,
            Capabilities=['CAPABILITY_IAM'],
            Parameters=[
                {
                    'ParameterKey': 'AutoSpottingLambdaARN',
                    'ParameterValue': lambda_arn,
                },
            ],
        )
        print(response)
    except botocore.exceptions.ClientError:
        raise


def delete_stack(region):
    client = boto3.client('cloudformation', region)

    try:
        response = client.delete_stack(StackName=STACK_NAME)
        print(response)

        waiter = client.get_waiter('stack_delete_complete')
        waiter.wait(
            StackName=STACK_NAME,
            WaiterConfig={'Delay': 5}
        )
    except botocore.exceptions.ClientError:
        raise


def handle_create(event):
    ec2 = boto3.client('ec2')
    lambda_arn = event['ResourceProperties']['LambdaARN']
    threads = []

    # create concurrently in all regions
    for region in ec2.describe_regions()['Regions']:
        process = Thread(target=create_stack, args=[
                         region['RegionName'], lambda_arn])
        process.start()
        threads.append(process)

    for process in threads:
        process.join()


def handle_delete(event):
    ec2 = boto3.client('ec2')
    threads = []

    # delete concurrently in all regions
    for region in ec2.describe_regions()['Regions']:
        process = Thread(target=delete_stack, args=[region['RegionName']])
        process.start()
        threads.append(process)

    for process in threads:
        process.join()


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

# Informs CloudFormation about the state of the custom resource


def send(event, context, responseStatus, responseData, physicalResourceId=None, noEcho=False):
    responseUrl = event['ResponseURL']

    print(responseUrl)

    responseBody = {}
    responseBody['Status'] = responseStatus
    responseBody[
        'Reason'] = 'See the details in CloudWatch Log Stream: ' + context.log_stream_name
    responseBody[
        'PhysicalResourceId'] = physicalResourceId or context.log_stream_name
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
