#!/bin/bash

set -e
shopt -s nullglob

export LC_ALL=C
ACCOUNT_ID=$1
ACCOUNT_SECRET=$2
IA=$3

DEFAULT_BRANCH_NAME="scionlab"
REMOTE_REPO="origin"
SCION_COORD_URL="https://coord.scionproto.net"

echo "Invoking update script with $ACCOUNT_ID $ACCOUNT_SECRET $IA"

UPDATE_BRANCH=$(curl --fail "${SCION_COORD_URL}/api/as/queryUpdateBranch/${ACCOUNT_ID}/${ACCOUNT_SECRET}?IA=${IA}" || true)

if [  -z "$UPDATE_BRANCH"  ]
then
    echo "No branch name has been specified, using default value ${DEFAULT_BRANCH_NAME}. "
    UPDATE_BRANCH=$DEFAULT_BRANCH_NAME
fi

echo "Update branch is: ${UPDATE_BRANCH}"

cd $SC

git_username=$(git config user.name || true)
if [ -z "$git_username" ]
then
    echo "GIT user credentials not set, configuring defaults"
    git config --global user.name "Scion User" 
    git config --global user.email "scion@scion-architecture.net"
    git config --global url.https://github.com/.insteadOf git@github.com:
fi
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
    ./scion.sh clean || true
    ./scion.sh run
fi

RESULT=$(curl -X POST "${SCION_COORD_URL}/api/as/confirmUpdate/${ACCOUNT_ID}/${ACCOUNT_SECRET}?IA=${IA}")
echo "Done, got response from server: ${RESULT}"