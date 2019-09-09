#!/bin/bash
# SCION upgrade version 0.10

wget https://raw.githubusercontent.com/netsec-ethz/scion-coord/master/scion_upgrade_script.sh -O upgrade.sh
chmod +x upgrade.sh

./upgrade.sh
