#!/bin/bash

FORMAT='%d-%b-%Y'

if [ "$1" = 'nightly' ]
then
  echo `date +$FORMAT -d "+2 months"`
elif [ "$1" = 'custom' ]
then
  echo `date +$FORMAT -d "+100 years"`
fi
