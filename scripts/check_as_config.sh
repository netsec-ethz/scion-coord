#!/bin/bash

# This script asks the Coordinator for its configuration, using the IA account ID and secret and configuration
# version stored inside the gen folder. The Coordinator can reply saying this AS has the latest version, reply with
# a new configuration package or reply by requesting this AS to remove its configuration.
# This script will stop SCION and start it again with the new configuration if needed.
# This script also stops and starts the VPN client accordingly, depending on the existing configuration and the new
# one.

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


onexit() {
    CODE=$?
    if [ $CODE -ne 0 ]; then
        exit $CODE
    fi
    [ ! -z "$TEMPTGZ" ] && rm -f "$TEMPTGZ"
    [ ! -z "$TMP" ] && rm -rf "$TMP"
}
trap onexit EXIT

if [ -f "$SC/gen/coord_url" ]; then
    SCION_COORD_URL=$(cat "$SC/gen/coord_url")
    echo "Special Coordinator in use. URL: $SCION_COORD_URL"
fi
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
TEMPTGZ=$(mktemp -u /tmp/gen-data-XXXXXX.tgz)
HTTP_CODE=$(curl -s -w "%{http_code}" "$SCION_COORD_URL/api/as/getASData/$ACC_ID/$ACC_PW/$IA?${LOCAL_VER}" --output ${TEMPTGZ}) || { echo "curl failed with code $?. Aborting"; exit 1; }
if [ $HTTP_CODE -eq 304 ]; then
    echo "Existing AS configuration is up to date"
    exit 0
elif [ $HTTP_CODE -eq 205 ]; then
    echo "AS should be detached"
    if [ ! -d $SC/gen/ISD* ] && [ ! -d $SC/gen/dispatcher ]; then
        echo "AS is already detached"
        exit 0
    fi
elif [ $HTTP_CODE -eq 200 ]; then
    echo "There is a new AS configuration"
else
    echo "Unhandled status code received from Coordinator: $HTTP_CODE"
    [ -f $TEMPTGZ ] && file $TEMPTGZ | grep ASCII >/dev/null 2>&1 &&  echo "Coordinator message is:" && cat $TEMPTGZ
    exit 1
fi
# we received a new AS configuration or AS was detached
cd "$SC"
./scion.sh stop || true

TMP=$(mktemp -d)
cd $TMP
if [ $HTTP_CODE -eq 200 ]; then
    echo "Expanding tar file in $TMP"
    tar xf $TEMPTGZ
    # copy gen folder:
    DIR=$(ls)
    if [ $(echo $DIR | wc -l) -ne 1 ]; then
        # failed assertion
        echo "Expected exactly one entry in the downloaded tar file $TEMPTGZ but found a different number. Aborting"
        exit 1
    fi
    DIR=$(realpath $TMP/$DIR)
else # must be code 205: detaching AS
    cp -rL "$SC/gen" .
    DIR="$TMP"
    rm -rf $DIR/gen/ISD* $DIR/gen/dispatcher
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
    sudo systemctl stop "openvpn@client.service" || true
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
rm -f ./gen-cache/* # because we could have even changed IDs !!
# we are not responsible for (re)building SCION:
./scion.sh start nobuild
