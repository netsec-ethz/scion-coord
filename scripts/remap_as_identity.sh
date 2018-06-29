#!/bin/bash

set -e

basedir=$(dirname $(realpath $0))
IA=$(cat $SC/gen/ia)

if [ ! -f "$basedir/remap_as_identity.py" ]; then
    wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scripts/remap_as_identity.py -O "$basedir/remap_as_identity.py"
fi
cd "$SC"
./scion.sh stop
~/.local/bin/supervisorctl -c supervisor/supervisord.conf shutdown
./tools/zkcleanslate 
./scion.sh clean || true

python3 $basedir/remap_as_identity.py --ia "$IA"

rm ./gen-cache/*
bash -c 'yes | GO_INSTALL=true ./env/deps' || echo "ERROR: Dependencies failed. Starting SCION might fail!"
./scion.sh start
