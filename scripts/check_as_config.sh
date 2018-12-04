#!/bin/bash

set -e

SCION_COORD_URL="https://www.scionlab.org"
# SCION_COORD_URL="http://localhost:8080"

BASEDIR=$(dirname $(realpath $0))

if [ ! -f $SC/gen/ia ] || [ ! -f $SC/gen/account_id ] || [ ! -f $SC/gen/account_secret ]; then
    echo "A required file is missing!"
    echo "Check that the directory \"$SC\" contains: \"ia\", \"account_id\" and \"account_secret\". "
    echo "Redownload your SCIONLab AS configuration and redeploy otherwise."
    exit 1
fi

IA=$(cat $SC/gen/ia | sed 's/_/\:/g')
ACC_ID=$(cat $SC/gen/account_id)
ACC_PW=$(cat $SC/gen/account_secret)
[ -f $SC/gen/coord_conf.ver ] && LOCAL_VER=$(cat $SC/gen/coord_conf.ver) || LOCAL_VER=0

cd /tmp
rm -f gen-data.tgz
curl -s -D - "$SCION_COORD_URL/api/as/getASData/$ACC_ID/$ACC_PW/$IA?local_version=$LOCAL_VER" --output gen-data.tgz





# TODO check status out of the curl output. 200 => NEWGEN=1
NEWGEN=1

if [ $NEWGEN -ne 1 ]; then
    echo "Existing AS configuration is up to date"
    exit 0
fi
./scion.sh stop || true

TMP=$(mktemp -d)
cd $TMP
echo "Expanding tar file in $TMP"
tar xf /tmp/gen-data.tgz
# copy gen folder:
DIR=$(ls)
if [ $(echo $DIR | wc -l) -ne 1 ]; then
    # failed assertion
    echo "Expected exactly one entry in the downloaded tar file /tmp/gen-data.tgz but found a different number. Aborting"
    exit 1
fi
DIR=$(realpath $TMP/$DIR)
cd $SC
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
if [ -d gen ]; then
    mv gen "gen.bak-$TIMESTAMP" || { echo "Failed to rename gen folder. Aborting."; exit 1; }
fi
mv $DIR/gen .
echo "gen folder moved into place"
# copy VPN client file
if [ -f /etc/openvpn/client.conf ] && [ -f $DIR/client.conf ]; then
    echo "Saving a backup copy of /etc/openvpn/client.conf"
    sudo cp /etc/openvpn/client.conf "/etc/openvpn/client.conf.bak-$TIMESTAMP"
    sudo systemctl stop "openvpn@client.service" && sleep 2 || true
else
    echo "No /etc/openvpn/client.conf file found or not using VPN in this AS configuration. Step skipped."
fi
if [ -f $DIR/client.conf ]; then
    echo "Using VPN in this AS configuration"
    sudo mv $DIR/client.conf /etc/openvpn/
    sudo systemctl start "openvpn@client.service" && sleep 2 || echo "Failed to start VPN. Please start it manually. Your AS may fail to start correctly."
else
    echo "Not using VPN in this AS configuration. Step skipped."
fi
# now reload SCION
echo "Reloading AS configuration"
./supervisor/supervisor.sh reload
./tools/zkcleanslate || true
rm ./gen-cache/* || true
./scion.sh start nobuild
