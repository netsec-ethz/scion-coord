#!/bin/bash
# Copyright 2018 ETH Zurich
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

GITREPO="origin"
GITBRANCH="scionlab"
BACKUPBRANCH="backup_upgrade1"
#MAKECHANGES=0
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

output=$(git remote update) && success=1 || success=0
if [ $success != 1 ]; then
    echo "Error updating remote: $output"
    exit 1
fi

output=$(git fetch "$GITREPO" "$GITBRANCH" 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "Error fetching branch: $output"
    exit 1
fi

if [ $(git rev-parse HEAD) != $(git rev-parse "$GITREPO/$GITBRANCH") ]; then
  if git branch | grep "$BACKUPBRANCH"; then
      echo "Branch $BACKUPBRANCH exists already. Aborting."
      exit 1
  fi
fi

if [ $(git status --untracked-files=no --short | wc -l) != 0 ]; then
    echo "Local copy of repository modified:"
    git status --untracked-files=no
    echo "Aborting."
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
output=$(./supervisor/supervisor.sh shutdown 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION supervisor shutdown failed:"
    echo "$output"
    # exit 1
fi

# Only 
if [ $(git rev-parse HEAD) != $(git rev-parse "$GITREPO/$GITBRANCH") ]; then
  git branch "$BACKUPBRANCH"
  output=$(git reset --hard "$GITREPO/$GITBRANCH" 2>&1) && success=1 || success=0
  if [ $success != 1 ]; then
      echo "Problem updating copy:"
      echo "$output"
      exit 1
  fi
fi

echo "Updated"
git status -v

echo "Install dependencies"
# because upgrading to SCIONLab 2019-01 will fail if installed, remove it:
sudo apt-get remove -y parallel
bash -c 'yes | GO_INSTALL=true ./env/deps' || exit 1

echo "Starting build."
output=$(./scion.sh clean 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION clean failed:"
    echo "$output"
    # exit 1
fi
output=$(./scion.sh build 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION build failed:"
    echo "$output"
    # exit 1
fi
