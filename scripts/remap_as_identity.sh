#!/bin/bash

set -e


basedir=$(dirname $(realpath $0))
OLDIA=$(cat $SC/gen/ia | sed 's/_/\:/g')


if [ ! -f "$basedir/remap_as_identity.py" ]; then
    wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scripts/remap_as_identity.py -O "$basedir/remap_as_identity.py"
fi
cd "$SC"
./scion.sh stop || true

PYTHONPATH="$SC/python:$SC" python3 $basedir/remap_as_identity.py --ia "$OLDIA"
NEWIA=$(cat $SC/gen/ia | sed 's/_/\:/g')

if [ "$NEWIA" != "$OLDIA" ]; then
    echo "remap, IA changed."
    ./supervisor/supervisor.sh reload
    ./tools/zkcleanslate || true
    rm ./gen-cache/* || true
    ./scion.sh clean || true
    mv go/vendor/vendor.json /tmp && rm -r go/vendor && mkdir go/vendor || true
    mv /tmp/vendor.json go/vendor/ || true
    pushd go >/dev/null
    govendor sync
    popd >/dev/null
    bash -c 'yes | GO_INSTALL=true ./env/deps >/dev/null 2>&1' || echo "ERROR: Dependencies failed. Starting SCION might fail!"
    ./scion.sh build >/dev/null 2>&1 || echo "Building SCION failed. Starting SCION might fail!"
else
    echo "remap, IA is the same"
fi
./scion.sh start
