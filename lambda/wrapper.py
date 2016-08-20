"""
This code downloads and executes the autospotting golang binary, but only if
it is missing from the current directory, in case of a new deployment or if
Lambda moved our code to a new instance.
"""

import json
import os.path
import sys

from urlparse import urljoin
from urllib2 import URLError, urlopen
from subprocess import call, PIPE, STDOUT, check_output

URL_PATH = 'https://cdn.cloudprowess.com/dv/'


def lambda_handler(event, context):
    """ Main entry point for Lambda """
    print 'Received event: ' + json.dumps(event, indent=2)
    print "Context log stream: " + context.log_stream_name

    try:
        filename = get_latest_agent_filename()
        download_agent_if_missing(filename)
        run_agent(filename)

    except URLError as ex:
        print 'Error: ', ex


def get_latest_agent_filename():
    """Determines the filename of the latest released agent golang binary"""
    return urlopen(
        urljoin(
            URL_PATH,
            'latest_agent'
        )
    ).read().strip()


def download_agent_if_missing(filename):
    """ Downloads the agent if missing from the current Lambda run """
    if file_missing(filename):
        print filename+'is missing, downloading it first'
        download(filename)


def file_missing(filename):
    """Checks file for existence"""
    return not os.path.isfile(filename)


def download(filename):
    """ Downloads a file from the base URL path into /tmp/<filename> """
    print "Downloading", filename
    file_content = urlopen(
        urljoin(URL_PATH, filename)
    )
    write_data_to_file(
        file_content.read(),
        os.path.join(
            '/tmp',
            filename
            )
        )


def write_data_to_file(data, filename):
    """ Writes data to filename """
    with open(filename, 'wb') as outfile:
        outfile.write(data)


def run_agent(filename):
    """ Runs the agent witn the required params """
    print "Running", filename

    binary_path = os.path.join('/tmp', filename)

    os.chmod(binary_path, 0755)
    call([binary_path], stderr=STDOUT)
