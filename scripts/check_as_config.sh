#!/bin/bash

set -e

SCION_COORD_URL="https://www.scionlab.org"

force=0
usage="$(basename $0) [-f] [-h]

where:
    -h      This help
    -f      (force) Ignore the configuration version
            and get the configuration from Coordinator"
while getopts "hf" opt; do
    case $opt in
    h)
        echo "$usage"
        exit 0
        ;;
    f)
        force=1
        ;;
    \?)
        echo "$usage"
        exit 1
        ;;
    *)
        echo "$usage"
        exit 1
        ;;
    esac
done


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
[ $force -eq 1 ] && LOCAL_VER="force=1" || LOCAL_VER="local_version=$LOCAL_VER"

cd /tmp
rm -f gen-data.tgz
HTTP_CODE=$(curl -s -w "%{http_code}" "$SCION_COORD_URL/api/as/getASData/$ACC_ID/$ACC_PW/$IA?local_version=${LOCAL_VER}${FORCEFLAG}" --output gen-data.tgz)
if [ $HTTP_CODE -eq 304 ]; then
    echo "Existing AS configuration is up to date"
    exit 0
elif [ $HTTP_CODE -eq 205 ]; then
    echo "AS is detached"
elif [ $HTTP_CODE -eq 200 ]; then
    echo "New AS configuration"
else
    echo "Unhandled status code received from Coordinator: $HTTP_CODE"
    [ -f /tmp/gen-data.tgz ] && file /tmp/gen-data.tgz | grep ASCII >/dev/null 2>&1 &&  echo "Coordinator message is:" && cat /tmp/gen-data.tgz
    exit 1
fi
# we received a new AS configuration or AS was detached
cd "$SC"
./scion.sh stop || true

TMP=$(mktemp -d)
cd $TMP
if [ $HTTP_CODE -eq 200 ]; then
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
else # detaching AS
    cp -r "$SC/gen" .
    DIR="$TMP"
    rm -rf $DIR/gen/ISD* $DIR/gen/dispatcher
    echo "Guessing the new AS configuration version to be $((LOCAL_VER + 1))"
    [ -f "$DIR/gen/coord_conf.ver" ] && echo $((LOCAL_VER + 1)) > "$DIR/gen/coord_conf.ver"
fi
cd $SC
# backup_gen
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
if [ -d gen ]; then
    mv gen "gen.bak-$TIMESTAMP" || { echo "Failed to rename gen folder. Aborting."; exit 1; }
fi
mv $DIR/gen .
echo "gen folder moved into place"

# copy VPN client file
if [[ -f /etc/openvpn/client.conf  && (-f $DIR/client.conf || $HTTP_CODE -eq 205) ]]; then
    echo "Saving a backup copy of /etc/openvpn/client.conf"
    sudo mv /etc/openvpn/client.conf "/etc/openvpn/client.conf.bak-$TIMESTAMP"
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
rm -f ./gen-cache/*
./scion.sh start nobuild
