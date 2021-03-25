# Copyright (c) 2016-2020 Cristian Măgherușan-Stanciu
# Licensed under the Open Software License version 3.0
'''
This code implements a Lambda function handler that implements the following
functionality for a given AWA AutoScaling Group:
- manage ASG maximum capacity
- suspend/resume AutoScaling processes that might interfere with AutoSpotting
'''
import logging
from time import sleep

from botocore.exceptions import ClientError
from boto3 import client

logger = logging.getLogger()
logger.setLevel(logging.INFO)

suspended_processes = ['Terminate', 'AZRebalance', 'Launch']


def change_max_size(svc, asg, variation):
    ''' Update AutoScaling Group maximum capacity '''

    try:
        response = svc.describe_auto_scaling_groups(
            AutoScalingGroupNames=[asg],
        )

        if not response['AutoScalingGroups']:
            raise ClientError(f'ASG {asg} not found!')
    except ClientError as client_error:
        logger.error('Failed to describe %s: %s', asg, client_error)
        return False

    current_size = response['AutoScalingGroups'][0]['MaxSize']
    new_size = int(current_size) + int(variation)

    logger.info('ASG %s Current size: %s, extending to %s',
                asg, current_size, new_size)

    try:
        response = svc.update_auto_scaling_group(
            AutoScalingGroupName=asg,
            MaxSize=new_size,
        )
    except ClientError as client_error:
        logger.error('Failed to change ASG %s MaxSize to %s: %s',
                     asg, new_size, client_error)
        return False

    logger.info('ASG %s MaxSize changed to %s', asg, new_size)
    return True


def suspend(svc, asg, tag):
    ''' Suspend processes of a given AutoScaling group. '''
    try:
        svc.suspend_processes(
            AutoScalingGroupName=asg,
            ScalingProcesses=suspended_processes,
        )
        return True
    except ClientError as client_error:
        logger.error(
            'Failed suspend process on ASG %s: %s', asg, client_error)
        return False
    else:
        try:
            svc.create_or_update_tags(Tags=[tag])
            return True
        except ClientError as client_error:
            logger.error(
                'Failed to tag ASG %s for suspend termination process: %s',
                asg, client_error)
            return False


def resume(svc, asg, tag):
    ''' Resume the processes of a given AutoScaling group. '''
    try:
        svc.resume_processes(
            AutoScalingGroupName=asg,
            ScalingProcesses=suspended_processes,
        )
        svc.delete_tags(Tags=[tag])
        return True
    except ClientError as client_error:
        logger.error(
            'Failed resume process on ASG %s: %s', asg, client_error)
        return False


def suspend_resume(svc, asg, action):
    ''' Suspend or Resume the processes of a given AutoScaling group. '''
    tag = {
        'ResourceId': asg,
        'Key': 'autospotting_suspended_processes',
        'ResourceType': 'auto-scaling-group',
        'PropagateAtLaunch': False,
        'Value': ','.join(suspended_processes),
    }
    if action == 'suspend':
        return suspend(svc, asg, tag)
    if action == 'resume':
        sleep(120)
        return resume(svc, asg, tag)
    return False


def handler(event, _):
    ''' Lambda function handler '''
    region = event['region']
    svc = client('autoscaling', region_name=region)
    asg = event['asg']

    if 'variation' in event:
        variation = event['variation']
        logger.info('ASG %s Extending by to %s', asg, variation)
        return change_max_size(svc, asg, variation)

    instance_id = event['instanceid']
    action = event['action']
    logger.info('ASG %s Taking action: %s for %s',
                asg, action, instance_id)
    return suspend_resume(svc, asg, action)
