# Copyright (c) 2016-2020 Cristian Măgherușan-Stanciu
# Licensed under the Open Software License version 3.0

import logging
from boto3 import client

logger = logging.getLogger()
logger.setLevel(logging.INFO)


def change_max_size(svc, asg, variation):
    try:
        response = svc.describe_auto_scaling_groups(
            AutoScalingGroupNames=[asg],
        )

        if not response['AutoScalingGroups']:
            raise Exception(f'ASG {asg} not found!')
    except Exception as e:
        logger.error(f'Failed to describe {asg}: {e}')
        return False

    currentSize = response['AutoScalingGroups'][0]['MaxSize']
    newSize = int(currentSize) + int(variation)

    logger.info(
        f'ASG {asg} Current size: {currentSize}, extending to {newSize}')

    try:
        response = svc.update_auto_scaling_group(
            AutoScalingGroupName=asg,
            MaxSize=newSize,
        )
    except Exception as e:
        logger.error(
            f'Failed to change ASG {asg} MaxSize to {newSize}: {e}')
        return False

    logger.info(f'ASG {asg} MaxSize changed to {newSize}')
    return True


def suspend(svc, asg, tag):
    try:
        svc.suspend_processes(
            AutoScalingGroupName=asg,
            ScalingProcesses=['Terminate'],
        )
        return True
    except Exception as e:
        logger.error(
            f'Failed suspend process on ASG {asg}: {e}')
        return False
    else:
        try:
            svc.create_or_update_tags(Tags=[tag])
            return True
        except Exception as e:
            logger.error(
                f'Failed to tag ASG {asg} for suspend ' +
                'process by {instanceId}: {e}')
            return False


def resume(svc, asg, tag, instanceId):
    try:
        response = svc.describe_tags(
            Filters=[
                {
                    'Name': 'auto-scaling-group',
                    'Values': [asg],
                },
                {
                    'Name': 'key',
                    'Values': ['autospotting_suspend_processes_by'],
                },
                {
                    'Name': 'value',
                    'Values': [instanceId],
                }
            ]
        )
        if response['Tags']:
            svc.resume_processes(
                AutoScalingGroupName=asg,
                ScalingProcesses=['Terminate'],
            )
            svc.delete_tags(Tags=[tag])
            return True
    except Exception as e:
        logger.error(
            f'Failed resume process on ASG {asg}: {e}')
        return False


def suspend_resume(svc, asg, action, instanceId):
    tag = {
        'ResourceId': asg,
        'Key': 'autospotting_suspend_processes_by',
        'ResourceType': 'auto-scaling-group',
        'PropagateAtLaunch': False,
        'Value': instanceId,
    }
    if action == 'suspend':
        return suspend(svc, asg, tag)
    elif action == 'resume':
        return resume(svc, asg, tag, instanceId)


def handler(event, context):
    region = event['region']
    svc = client('autoscaling', region_name=region)
    asg = event['asg']

    if 'variation' in event:
        variation = event['variation']
        logger.info(f'ASG {asg} Extending by to {variation}')
        return change_max_size(svc, asg, variation)
    else:
        instanceId = event['instanceid']
        action = event['action']
        logger.info(f'ASG {asg} Taking action: {action} for {instanceId}')
        return suspend_resume(svc, asg, action, instanceId)
