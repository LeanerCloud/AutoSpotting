'''
    Creates regional CloudFormation stacks that trigger the main AutoSpotting
    Lambda function
'''

import traceback

from json import dumps
from sys import exc_info
from threading import Thread

from boto3 import client
from botocore.exceptions import ClientError
from botocore.vendored import requests
from botocore.vendored.requests.exceptions import RequestException

SUCCESS = "SUCCESS"
FAILED = "FAILED"
STACK_NAME = 'AutoSpottingRegionalResources'
TEMPLATE_URL = \
    'https://s3.amazonaws.com/cloudprowess/nightly/regional_template.yaml'


def create_stack(region, lambda_arn):
    '''Creates a regional CloudFormation stack'''
    cfn = client('cloudformation', region)
    response = {}

    # delete stack if it already exists from a previous run
    try:
        delete_stack(region)
    except ClientError:
        pass

    response = cfn.create_stack(
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


def delete_stack(region):
    ''' Deletes a regional CloudFormation stack'''
    cfn = client('cloudformation', region)

    response = cfn.delete_stack(StackName=STACK_NAME)
    print(response)

    waiter = cfn.get_waiter('stack_delete_complete')
    waiter.wait(
        StackName=STACK_NAME,
        WaiterConfig={'Delay': 5}
    )


def handle_create(event):
    ''' Creates regional stacks in all available AWS regions concurrently '''
    ec2 = client('ec2')
    lambda_arn = event['ResourceProperties']['LambdaARN']
    threads = []

    # create concurrently in all regions
    for region in ec2.describe_regions()['Regions']:
        process = Thread(target=create_stack,
                         args=[region['RegionName'], lambda_arn])
        process.start()
        threads.append(process)

    for process in threads:
        process.join()


def handle_delete():
    ''' Concurrently deletes regional stacks in all available AWS regions '''
    ec2 = client('ec2')
    threads = []

    # delete concurrently in all regions
    for region in ec2.describe_regions()['Regions']:
        process = Thread(target=delete_stack, args=[region['RegionName']])
        process.start()
        threads.append(process)

    for process in threads:
        process.join()


def handler(event, context):
    ''' Lambda function entry point '''
    try:
        if event['RequestType'] == 'Create':
            handle_create(event)
        if event['RequestType'] == 'Delete':
            handle_delete()
        send(event, context, SUCCESS, {})
    except ClientError:
        traceback.print_exc()
        print("Unexpected error:", exc_info()[0])
        send(event, context, FAILED, {})


def send(event, context, response_status, response_data):
    ''' Informs CloudFormation about the state of the custom resource '''
    response_url = event['response_url']

    print(response_url)

    response_body = {}
    response_body['Status'] = response_status
    response_body['Reason'] = \
        'See the details in CloudWatch Log Stream: ' + context.log_stream_name
    response_body['PhysicalResourceId'] = context.log_stream_name
    response_body['StackId'] = event['StackId']
    response_body['RequestId'] = event['RequestId']
    response_body['LogicalResourceId'] = event['LogicalResourceId']
    response_body['NoEcho'] = None
    response_body['Data'] = response_data

    json_response_body = dumps(response_body)

    print("Response body:\n" + json_response_body)

    headers = {
        'content-type': '',
        'content-length': str(len(json_response_body))
    }

    try:
        response = requests.put(response_url,
                                data=json_response_body,
                                headers=headers)
        print("Status code: " + response.reason)
    except RequestException as exception:
        print("send(..) failed executing requests.put(..): " + str(exception))
