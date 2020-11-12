# Copyright (c) 2016-2020 Cristian Măgherușan-Stanciu
# Licensed under the Open Software License version 3.0

import logging
from boto3 import client

logger = logging.getLogger()
logger.setLevel(logging.INFO)


def change_max_size(svc, region, asg, variation):
    try:
        response = svc.describe_auto_scaling_groups(
            AutoScalingGroupNames=[asg],
        )

        if not response['AutoScalingGroups']:
            raise Exception(f'AutoScalingGroup {asg} not found!')
    except Exception as e:
        logger.error(f'Failed to describe {asg}: {e}')
        return False

    currentSize = response['AutoScalingGroups'][0]['MaxSize']
    newSize = int(currentSize) + int(variation)

    try:
        response = svc.update_auto_scaling_group(
            AutoScalingGroupName=asg,
            MaxSize=newSize,
        )
    except Exception as e:
        logger.error(
            f'Failed to change AutoScalingGroup {asg} MaxSize to {newSize}: {e}')
        return False

    logger.info(f'AutoScalingGroup {asg} MaxSize changed to {newSize}')
    return True


def suspend_resume(svc, region, asg, action, instanceId):
    tag = {
        'ResourceId': asg,
        'Key': 'autospotting_suspend_processes_by',
        'ResourceType': 'auto-scaling-group',
        'PropagateAtLaunch': False,
        'Value': instanceId,
    }
    if action == 'suspend':
        try:
            svc.suspend_processes(
                AutoScalingGroupName=asg,
                ScalingProcesses=['Terminate'],
            )
        except Exception as e:
            logger.error(
                f'Failed suspend process on AutoScalingGroup {asg}: {e}')
            return False
        else:
            try:
                svc.create_or_update_tags(Tags=[tag])
                return True
            except Exception as e:
                logger.error(
                    f'Failed to tag AutoScalingGroup {asg} for suspend process by {instanceId}: {e}')
                return False
    if action == 'resume':
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
        except Exception as e:
            logger.error(
                f'Failed resume process on AutoScalingGroup {asg}: {e}')
            return False


def handler(event, context):
    region = event['region']
    svc = client('autoscaling', region_name=event['region'])
    asg = event['asg']
    if 'variation' in event:
        variation = event['variation']
        return change_max_size(svc, region, asg, variation)
    else:
        instanceId = event['instanceid']
        action = event['action']
        return suspend_resume(svc, region, asg, action, instanceId)
