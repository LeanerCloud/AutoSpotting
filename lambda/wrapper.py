"""
This code downloads and executes the autospotting golang binary, but only if
it is missing from the current directory, in case of a new deployment or if
Lambda moved our code to a new instance.
"""

import json

from subprocess import call, STDOUT

BINARY = './autospotting_lambda'
JSON_INSTANCES = 'instances.json'
GIT_SHA = 'GIT_SHA'

def lambda_handler(event, context):
    """ Main entry point for Lambda """

    with open(GIT_SHA, 'r') as sha:
        print 'Starting AutoSpotting, built from the git SHA', sha.read()

    print 'Received event: ' + json.dumps(event, indent=2)

    print "Running", BINARY, JSON_INSTANCES
    call([BINARY, JSON_INSTANCES], stderr=STDOUT)


if __name__ == '__main__':
    lambda_handler(None, None)
