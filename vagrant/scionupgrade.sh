#!/bin/bash

ACCOUNT_ID=$(<"$SC/gen/account_id")
ACCOUNT_SECRET=$(<"$SC/gen/account_secret")
IA=$(<"$SC/gen/ia")

#TODO: replace URL with correct address
wget https://gist.githubusercontent.com/xabarass/def1f861f0fbb1f51d7479d23e239c6f/raw/0d3220b63d82b54df87ed412c5a4db876d2c438c/upgrade.sh
chmod +x upgrade.sh

./upgrade.sh $ACCOUNT_ID $ACCOUNT_SECRET $IA

