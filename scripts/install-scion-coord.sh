#!/bin/bash

set -e

basedir="$(realpath $(dirname $(realpath $0))/../)"

# check binary
if [ ! -x "$basedir/scion-coord" ]; then
    echo "SCION Coordinator binary file scion-coord is missing. Building."
    pushd "$basedir" >/dev/null
    go build
    if [ -x "scion-coord" ]; then
        echo "Built."
    else
        echo "Still didn't find the binary. Abort."
        exit 1
    fi
    popd >/dev/null
fi

# TODO: in the future we probably want to perform checks against the DB, the credentials, 
# the scionLabConfigs/easy-rsa and basically anything that we perform manually

# check service file
if [ ! -f "$basedir/scripts/files/scion-coord.service" ] 
   || [ ! -f "$basedir/scripts/files/unit-status-mail@.service" ] 
   || [ ! -f "$basedir/scripts/files/emailer.py" ]; then
    echo "Missing service file"
    exit 1
fi
if [ ! -d "/etc/systemd/system" ]; then
    echo "Cannot find the destination directory for the service file. /etc/systemd/system not found. Abort."
    exit 1
fi

sudo systemctl stop "scion-coord" || true
pushd "/etc/systemd/system" >/dev/null
if [ -f "scion-coord.service" ]; then
    echo "There exist a previous scion-coord.service file. Moving it to /tmp as a backup measure."
    sudo mv "scion-coord.service" "/tmp/scion-coord.service.bak.from-scion-coord-installer"
fi
tmpfile=$(mktemp)
cp "$basedir/scripts/scion-coord.service" "$tmpfile"
sed -i -- "s/_USER_/$USER/g" "$tmpfile"
sudo cp "$tmpfile" "scion-coord.service"
cp "$basedir/scripts/files/unit-status-mail@.service" "$tmpfile"
sed -i -- "s/_USER_/$USER/g" "$tmpfile"
sudo cp "$tmpfile" "unit-status-mail@.service"
popd >/dev/null
sudo cp "$basedir/scripts/files/emailer.py" "/usr/local/bin/emailer"

sudo systemctl daemon-reload
sudo systemctl start "scion-coord"
sleep 1
systemctl status "scion-coord" >/dev/null && fail=0 || fail=1
if [ $fail -ne 0 ]; then
    echo "Problem starting the service. Please check with systemctl status scion-coord or journalctl -xe"
    exit 1
fi
sudo systemctl enable "scion-coord"
echo "Success."
