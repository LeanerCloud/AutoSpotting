#!/bin/bash

FORMAT='%d-%b-%Y'

if [ "$1" = 'nightly' ]
then
  echo `date +$FORMAT -d "+2 months"`
else
  echo `date +$FORMAT -d "+1 year"`
fi
