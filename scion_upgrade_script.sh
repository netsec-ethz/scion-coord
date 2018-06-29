#!/bin/bash

set -e

# version of the systemd files:
SERVICE_CURRENT_VERSION="0.4"

# version less or equal. E.g. verleq 1.9 2.0.8  == true (1.9 <= 2.0.8)
verleq() {
    [  "$1" = "`echo -e "$1\n$2" | sort -V | head -n1`" ]
}

check_system_files() {
    # check service files:
    need_to_reload=0
    declare -a FILES_TO_CHECK=("/etc/systemd/system/scionupgrade.service"
                               "/etc/systemd/system/scionupgrade.timer")
    for f in "${FILES_TO_CHECK[@]}"; do
        VERS=$(grep "^# SCION upgrade version" "$f" | sed -n 's/^# SCION upgrade version \([0-9\.]*\).*$/\1/p')
        if ! verleq "$SERVICE_CURRENT_VERSION" "$VERS"; then
            # need to upgrade. (1) get the file with wget. (2) copy the file (3) reload systemd things
            bf=$(basename $f)
            tmpfile=$(mktemp)
            wget "https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/vagrant/$bf" -O "$tmpfile"
            sed -i "s/_USER_/$USER/g" "$tmpfile"
            sudo cp "$tmpfile" "$f"
            need_to_reload=1
        fi
    done
    if [ $need_to_reload -eq 1 ]; then
        [[ $(ps aux | grep ntpd | grep -v grep | wc -l) == 1 ]] && ntp_running=1 || ntp_running=0
        [[ $(grep -e 'start-stop-daemon\s*--start\s*--quiet\s*--oknodo\s*--exec\s*\/usr\/sbin\/VBoxService\s*--\s*--disable-timesync$' /etc/init.d/virtualbox-guest-utils |wc -l) == 1 ]] && host_synced=0 || host_synced=1
        if [ $host_synced != 0 ]; then
            echo "Disabling time synchronization via host..."
            sudo sed -i -- 's/^\(\s*start-stop-daemon\s*--start\s*--quiet\s*--oknodo\s*--exec\s*\/usr\/sbin\/VBoxService\)$/\1 -- --disable-timesync/g' /etc/init.d/virtualbox-guest-utils
            sudo systemctl daemon-reload
            sudo systemctl restart virtualbox-guest-utils
            sudo systemctl restart ntp || true # might fail if not installed yet
        fi
        if [ $ntp_running != 1 ]; then
            echo "Installing ntpd..."
            sudo apt-get install -y --no-remove ntp || true
            sudo systemctl enable ntp || true
            sudo systemctl restart ntp || true
        fi
        # don't attempt to stop the service as this script is a child of the service and will also be killed !
        # if really needed, specify KillMode=none in the service file itself
        sudo systemctl daemon-reload
    fi
}

is_id_standardized() {
    ia="$1"
    iaarray=(${ia//-/ })
    if [ "${iaarray[0]}" -lt "17" ]; then
        return 1
    else
        return 0
    fi
}


shopt -s nullglob

export LC_ALL=C
ACCOUNT_ID=$1
ACCOUNT_SECRET=$2
IA=$3

DEFAULT_BRANCH_NAME="scionlab"
REMOTE_REPO="origin"
SCION_COORD_URL="https://www.scionlab.org"

echo "Invoking update script with $ACCOUNT_ID $ACCOUNT_SECRET $IA"

# systemd files upgrade:
check_system_files

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
    ./tools/zkcleanslate

    echo "Reinstalling dependencies..."
    ./scion.sh clean || true
    bash -c 'yes | GO_INSTALL=true ./env/deps' || echo "ERROR: Dependencies failed. Starting SCION might fail!"

    echo "Starting SCION again..."
    ./scion.sh start
fi

RESULT=$(curl -X POST "${SCION_COORD_URL}/api/as/confirmUpdate/${ACCOUNT_ID}/${ACCOUNT_SECRET}?IA=${IA}")
echo "Done, got response from server: ${RESULT}"

if ! is_id_standardized "$IA" ; then
    echo "We need to map the addresses to the standard"
    wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scripts/remap_as_identity.sh -O remap_as_identity.sh || true
    bash remap_as_identity.sh
else
    echo "SCION IA follows standard."
fi
echo "Done."
