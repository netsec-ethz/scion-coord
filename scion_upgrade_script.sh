#!/bin/bash

set -e

# version of the systemd files:
SERVICE_CURRENT_VERSION="0.6"

# version less or equal. E.g. verleq 1.9 2.0.8  == true (1.9 <= 2.0.8)
verleq() {
    [ ! -z "$1" ] && [ ! -z "$2" ] && [ "$1" = `echo -e "$1\n$2" | sort -V | head -n1` ]
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
        if [ -d "/vagrant" ]; then # iff this is a VM
            echo "VM detected, checking time synchronization mechanism ..."
            [[ $(ps aux | grep ntpd | grep -v grep | wc -l) == 1 ]] && ntp_running=1 || ntp_running=0
            [[ $(grep -e 'start-stop-daemon\s*--start\s*--quiet\s*--oknodo\s*--exec\s*\/usr\/sbin\/VBoxService\s*--\s*--disable-timesync$' /etc/init.d/virtualbox-guest-utils |wc -l) == 1 ]] && host_synced=0 || host_synced=1
            if [ $host_synced != 0 ]; then
                echo "Disabling time synchronization via host..."
                sudo sed -i -- 's/^\(\s*start-stop-daemon\s*--start\s*--quiet\s*--oknodo\s*--exec\s*\/usr\/sbin\/VBoxService\)$/\1 -- --disable-timesync/g' /etc/init.d/virtualbox-guest-utils
                sudo systemctl daemon-reload
                sudo systemctl restart virtualbox-guest-utils
            fi
            if [ $ntp_running != 1 ]; then
                echo "Installing ntpd..."
                sudo apt-get install -y --no-remove ntp || true
                sudo systemctl enable ntp || true
            fi
            if ! egrep -- '^NTPD_OPTS=.*-g.*$' /etc/default/ntp >/dev/null; then
                sudo sed -i "s/^NTPD_OPTS='\(.*\)'/NTPD_OPTS=\'\\1\ -g'/g" /etc/default/ntp
            fi
            if ! egrep -- '^tinker panic 0' /etc/ntp.conf >/dev/null; then
                echo "set panic limit to 0 (disable)"
                echo -e "tinker panic 0\n" | sudo tee -a /etc/ntp.conf >/dev/null
            fi
            if ! egrep -- '^pool.*maxpoll.*$' /etc/ntp.conf >/dev/null; then
                echo "set minpoll 1 maxpoll 6 (increase frequency of ntpd syncs)"
                sudo sed -i 's/\(pool .*\)$/\1 minpoll 1 maxpoll 6/g' /etc/ntp.conf
            fi
            sudo systemctl restart ntp || true
            echo "ntpd restarted."
        fi
        # don't attempt to stop the scionupgrade service as this script is a child of it and will also be killed !
        # even with KillMode=none in the service file, restarting the service here would be really delicate, as it
        # could basically hang forever if the service files don't update the version number correctly, and we would
        # spawn a large number of processes one after the other, not doing anything but restarting the service.
        sudo systemctl daemon-reload
    fi
}

is_id_standardized() {
    ia="$1"
    echo $ia | grep _ >/dev/null && return 0
    iaarray=(${ia//-/ })
    if [ "${iaarray[1]}" -lt "1000000" ]; then
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

echo "Running git fetch $REMOTE_REPO $UPDATE_BRANCH &>/dev/null"
git fetch "$REMOTE_REPO" "$UPDATE_BRANCH" &>/dev/null
head_commit=$(git rev-parse "$REMOTE_REPO"/"$UPDATE_BRANCH")
if [ $(git branch "$UPDATE_BRANCH" --contains "$head_commit" 2>/dev/null | wc -l) -gt 0 ]; then
    echo "SCION version is already up to date!"
else
    git stash >/dev/null # just in case something was locally modified
    git reset --hard "$REMOTE_REPO"/"$UPDATE_BRANCH"
    # apply platform dependent patches, etc:
    ARCH=$(dpkg --print-architecture)
    echo -n "This architecture: $ARCH. "
    case "$ARCH" in
        "armhf")
            # current ARM patch:
            echo "Patching for ARM 32"
            curl https://gist.githubusercontent.com/juagargi/f007a3a80058895d81a72651af32cb44/raw/421d8bfecdd225a3b17a18ec1c1e1bf86c436b35/arm-scionlab-update2.patch | patch -p1
            git branch -D scionlab_autoupdate_patched 2>/dev/null|| true
            git add .
            git commit -m "SCIONLab autoupdate patch for ARM"
            ;;
        *)
            echo "No need to patch."
    esac
    echo "SCION code has been upgraded, stopping..."

    ./scion.sh stop || true
    ~/.local/bin/supervisorctl -c supervisor/supervisord.conf shutdown || true
    ./tools/zkcleanslate || true
    sudo systemctl restart zookeeper || true

    echo "Reinstalling dependencies..."
    ./scion.sh clean || true
    mv go/vendor/vendor.json /tmp && rm -r go/vendor && mkdir go/vendor || true
    mv /tmp/vendor.json go/vendor/ || true
    pushd go >/dev/null
    govendor sync || true
    popd >/dev/null
    bash -c 'yes | GO_INSTALL=true ./env/deps' || echo "ERROR: Dependencies failed. Starting SCION might fail!"

    echo "Starting SCION again..."
    ./scion.sh start || true
fi
RESULT=$(curl -X POST "${SCION_COORD_URL}/api/as/confirmUpdate/${ACCOUNT_ID}/${ACCOUNT_SECRET}?IA=${IA}") || true
echo "Done, got response from server: ${RESULT}"

if ! is_id_standardized "$IA" ; then
    echo "-----------------------------------------------------------------------------------"
    echo "We need to map the addresses to the standard"
    cd "/tmp"
    wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scripts/remap_as_identity.sh -O remap_as_identity.sh  && doremap=1 || doremap=0
    if [ "$doremap" == 1 ]; then
        bash remap_as_identity.sh || true
    else
        echo "Not yet mapping IA IDs"
    fi
else
    echo "SCION IA follows standard."
fi
echo "Done."
