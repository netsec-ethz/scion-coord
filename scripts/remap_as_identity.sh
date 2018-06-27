#!/bin/bash

set -e

basedir=$(dirname $(realpath $0))
IA=$(cat $SC/gen/ia)

if [ ! -f "$basedir/remap_as_identity.py" ]; then
    wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scripts/remap_as_identity.py -O "$basedir/remap_as_identity.py"
fi
python3 $basedir/remap_as_identity.py --ia "$IA"
