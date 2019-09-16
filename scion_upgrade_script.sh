#!/bin/bash

set -x
set -e
shopt -s nullglob

export LC_ALL=C

# the first and only thing we do is install the packaging system and remove the old upgrade mechanism
# NOT using aptdcon (aptdaemon) as trying to upgrade (aptdcon -u scionlab) will not pull any dependent packages

i=0
while sudo fuser /var/lib/dpkg/lock >/dev/null 2>&1; do
    sleep 10
    ((i=i+1))
    # wait no more than 10 minutes
    if ((i > 60)); then
        echo "Waited too long for /var/lib/dpkg/lock to be free. Bailing"
        exit 1
    fi
    echo "Waiting for /var/lib/dpkg/lock to be free..."
done

sudo apt-get install -y apt-transport-https
echo "deb [trusted=yes] https://packages.netsec.inf.ethz.ch/debian all main" | sudo tee /etc/apt/sources.list.d/scionlab.list
echo -e "`sudo  crontab -l`""\n`date -d '07:00 UTC' '+%M %H'` * * * apt-get update; apt-get install -y --only-upgrade scionlab" |sudo crontab
if [ -d "$SC" ]; then
    pushd "$SC"
    ./scion.sh stop || true
    popd
fi
# copy the config to /etc/scion
if [ -f "$SC/gen/scionlab-config.json" ]; then
    sudo mkdir -p /etc/scion/gen
    sudo cp "$SC/gen/scionlab-config.json" "/etc/scion/gen/"
fi

sudo apt-get update
sudo apt-get install -y scionlab  # this also installs and runs scionlab-config


sudo systemctl disable --now scion.service
sudo rm /etc/systemd/system/scion.service

sudo systemctl disable --now scionupgrade.timer
sudo systemctl disable scionupgrade.service  # do not stop scionupgrade.service (it's this script)
sudo rm /etc/systemd/system/scionupgrade.timer
sudo rm /etc/systemd/system/scionupgrade.service

sudo rm /usr/bin/scionupgrade.sh
sudo systemctl daemon-reload
sudo systemctl reset-failed

# in the VMs, we have manually installed ZK as dependency. Remove it
sudo apt-get purge -y zookeeper
