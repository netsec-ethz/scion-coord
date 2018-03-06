#!/bin/bash

set -e
shopt -s nullglob

ACCOUNT_ID=$1
ACCOUNT_SECRET=$2
IA=$3

DEFAULT_BRANCH_NAME="scionlab"
REMOTE_REPO="origin"
# TODO: Change me to official URL when we migrate
SCION_COORD_URL="https://scion-ad6.inf.ethz.ch"

echo "Invoking update script with $ACCOUNT_ID $ACCOUNT_SECRET $IA"

UPDATE_BRANCH=$(curl --fail "${SCION_COORD_URL}/api/as/queryUpdateBranch/${ACCOUNT_ID}/${ACCOUNT_SECRET}?IA=${IA}" || true)

if [  -z "$UPDATE_BRANCH"  ]
then
    echo "No branch name has been specified, using default value ${DEFAULT_BRANCH_NAME}. "
    UPDATE_BRANCH=$DEFAULT_BRANCH_NAME
fi

echo "Update branch is: ${UPDATE_BRANCH}"

cd $SC

git fetch "$REMOTE_REPO" "$UPDATE_BRANCH"
rebase_result=$(git rebase "${REMOTE_REPO}/${UPDATE_BRANCH}")

if [[ $rebase_result == *"is up to date"* ]]
then
    echo "SCION version is already up to date!"
else
    echo "SCION code has been upgraded, stopping..."

    ./scion.sh stop
    ~/.local/bin/supervisorctl -c supervisor/supervisord.conf shutdown

    echo "Reinstalling dependencies..."
    bash -c 'yes | GO_INSTALL=true ./env/deps'

    echo "Starting SCION again..."
    ./scion.sh run
fi

RESULT=$(curl -X POST "${SCION_COORD_URL}/api/as/confirmUpdate/${ACCOUNT_ID}/${ACCOUNT_SECRET}?IA=${IA}")
echo "Done, got response from server: ${RESULT}"