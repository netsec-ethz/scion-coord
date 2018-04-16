#!/bin/bash

set -e

GITREPO="origin"
GITBRANCH="scionlab"
BACKUPBRANCH="backup_upgrade1"
MAKECHANGES=0
# This script should be run on each of the ASes that are part of the infrastructure.
# Ideally called with ansible or forallASes -f


usage="$(basename $0) [-b branchname]

where:
    -b branchname:  Upgrade to branchname. Default value is scionlab"
while getopts ":b:h" opt; do
case $opt in
    h)
        echo "$usage"
        exit 0
        ;;
    b)
        GITBRANCH="$OPTARG"
        ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        echo "$usage" >&2
        exit 1
        ;;
    :)
        echo "Option -$OPTARG requires an argument." >&2
        echo "$usage" >&2
        exit 1
        ;;
esac
done
echo "Using branch $GITBRANCH"

# check we have $SC, and gen folder
if [ ! -d "$SC" ]; then
    echo "SC folder not found. SC=$SC"
    exit 1
fi
cd "$SC"
if [ ! -d "gen" ]; then
    echo "gen folder not found"
    exit 1
fi

# check we can sudo
sudo -S echo a >/dev/null </dev/null && success=1 || success=0
if [ $success != 1 ]; then
    echo "sudo failed"
    exit 1
fi

currentref=$(git rev-parse --abbrev-ref --symbolic-full-name @{u})
if [ "$currentref" != "${GITREPO}/${GITBRANCH}" ]; then 
    echo "Not currently using reference ${GITREPO}/${GITBRANCH}, but $currentref"
    exit 1
fi

if git branch | grep "$BACKUPBRANCH"; then
    echo "Branch $BACKUPBRANCH exists already. Aborting."
    exit 1
fi

if [ $(git status --untracked-files=no --short | wc -l) != 0 ]; then
    echo "Local copy of repository modified:"
    git status --untracked-files=no
    echo "Aborting."
    exit 1
fi

output=$(git fetch "$GITREPO" "$GITBRANCH" 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "Error fetching branch: $output"
    exit 1
fi

if [ "$MAKECHANGES" -ne 1 ]; then
    echo "Read only. Quitting now"
    exit 0
fi
echo "All checks OK."
########################################################## MODIFYING THE AS HERE ###############
git branch "$BACKUPBRANCH"
output=$(./scion.sh stop 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION stop failed:"
    echo "$output"
    exit 1
fi
# git pull
output=$(git pull --ff-only 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "Problem updating copy:"
    echo "$output"
    exit 1
fi
echo "Updated. Restarting."

output=$(./scion.sh clean 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION clean failed:"
    echo "$output"
    # exit 1
fi
output=$(./supervisor/supervisor.sh shutdown 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION supervisor shutdown failed:"
    echo "$output"
    # exit 1
fi
output=$(./tools/zkcleanslate --zk 127.0.0.1:2181 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "Zookeeper cleanslate failed:"
    echo "$output"
    # exit 1
fi
output=$(./scion.sh run 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION run failed:"
    echo "$output"
    # exit 1
fi


# all done
echo "Done."
exit 0
