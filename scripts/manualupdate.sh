#!/bin/bash
# SCION upgrade version 0.9

ACCOUNT_ID=$(<"$SC/gen/account_id")
ACCOUNT_SECRET=$(<"$SC/gen/account_secret")
IA=$(<"$SC/gen/ia")

if [ -z ACCOUNT_ID ] || [ -z ACCOUNT_SECRET ] || [ -z IA ]; then
    echo "Cannot find all necessary variables, aborting. IA=$IA, ACCOUNT_ID=$ACCOUNT_ID, ACCOUNT_SECRET=$ACCOUNT_SECRET"
    exit 1
fi

wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scion_upgrade_script.sh -O upgrade.sh
chmod +x upgrade.sh

./upgrade.sh $ACCOUNT_ID $ACCOUNT_SECRET $IA -m
