#!/bin/bash

set -e

GITREPO="juan"
GITBRANCH="stable_scionproto"
MAKECHANGES=1
# This script should be run on each of the ASes that are part of the infrastructure.
# Ideally called with ansible or forallASes -f

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

# check we can sudo. We have -e, will exit if not
echo sci0nLab | sudo -S -v &>/dev/null && success=1 || success=0
if [ $success != 1 ]; then
    echo "sudo failed"
    exit 1
fi

currentref=$(git rev-parse --abbrev-ref --symbolic-full-name @{u})
if [ "$currentref" != "${GITREPO}/${GITBRANCH}" ]; then 
    echo "Not currently using reference $GITBRANCH, but $currentref"
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
output=$(./supervisor/supervisor.sh reload 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION supervisor reload failed:"
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
