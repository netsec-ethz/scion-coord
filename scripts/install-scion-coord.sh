#!/bin/bash

# Installs the Coordinator and requirements, prepares the DB and runs the Coordinator.
# If invoked with the docker switch, it will just do the installation and return.

set -e

checkIfGitRepo() {
    DIR="$1"
    pushd "$DIR" &>/dev/null
    [ -d "$DIR/.git" ] || git rev-parse --git-dir &>/dev/null && popd &>/dev/null && return 0 || return 1
}

runSQL() {
    SQL=$1
    output=$(echo $SQL | $MYSQLCMD 2>&1) ; FAIL=$?
    output=$(echo "$output" | grep -Fv [Warning])
    echo "$output"
    return $FAIL
}


MYSQLCMD="mysql -u root -pdevelopment_pass"
NETSEC=${GOPATH:?}/src/github.com/netsec-ethz
SCIONCOORD="$NETSEC/scion-coord"
SCION="${GOPATH:?}/src/github.com/scionproto/scion"
CONFDIR="$HOME/scionLabConfigs"
basedir="$(realpath $(dirname $(realpath $0))/../)"
files="$basedir/files/install-coordinator"
inside_docker=0

usage="$(basename $0) [-n]
Installs the Coordinator in this machine
where:
    -d      Install inside docker container. This means no service files and not starting services in this script."
while getopts ":dh" opt; do
case $opt in
    h)
        echo "$usage"
        exit 0
        ;;
    d)
        inside_docker=1
        ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        echo "$usage" >&2
        exit 1
        ;;
    :)
        echo "Option -$OPTARG requires an argument." >&2
        echo "$usage" >&2
        exit 1
        ;;
esac
done

# coordinator sources
echo "Coordinator Installer [CoIn]: Installing Coordinator..."
mkdir -p "$NETSEC"
cd "$NETSEC"
if ! checkIfGitRepo "./scion-coord" ; then
    if [ -e "./scion-coord" ]; then
        echo "[CoIn]: Found existing scion-coord directory, but not under git. Moving directory to ./scion-coord.bak.from-scion-coord-installer"
        mv "./scion-coord" "./scion-coord.bak.from-scion-coord-installer"
    fi
    git config --global url.https://github.com/.insteadOf git@github.com:
    git clone --recursive git@github.com:netsec-ethz/scion-coord 
fi

# check go dependencies
command -v govendor >/dev/null 2>&1 || go get github.com/kardianos/govendor
cd "$SCIONCOORD/vendor"
govendor sync
echo "[CoIn]: done (Coordinator installed)."

# SCION
if [ -x "$basedir/scion_install_script.sh" ]; then
    echo "[CoIn]: Installing / Checking SCION ..."
    bash "$basedir/scion_install_script.sh"
    source ~/.profile
    echo "[CoIn]: done (SCION installed)."
fi

echo "[CoIn]: Installing other requirements..."
# check other requirements to be a Coordinator:
if ! python3 -c "from Crypto import Random" &>/dev/null; then
    echo "[CoIn]: Installing required crypto packages..."
    pip3 install --upgrade pycrypto
    echo "[CoIn]: Done installing required packages."
fi
if ! dpkg-query -s easy-rsa &> /dev/null ; then
    echo "[CoIn]: Installing easy-rsa"
    DEBIAN_FRONTEND="noninteractive" sudo apt-get install easy-rsa -y
fi
if ! dpkg-query -s openssl &> /dev/null ; then
    echo "[CoIn]: Installing openssl"
    DEBIAN_FRONTEND="noninteractive" sudo apt-get install openssl -y
fi

echo "[CoIn]: running apt-get update"
DEBIAN_FRONTEND="noninteractive" sudo apt-get update
if [ "$inside_docker" == 1 ]; then
    echo "[CoIn]: Installing MySQL client"
    DEBIAN_FRONTEND="noninteractive" sudo apt-get install -y "mysql-client-5.7"
elif ! dpkg-query -s mysql-server &> /dev/null ; then
    # MySQL DB step. If already installed, we assume it's already configured the way we want
    echo "[CoIn]: Installing MySQL server"
    echo "mysql-server-5.7 mysql-server/root_password password development_pass" | sudo debconf-set-selections
    echo "mysql-server-5.7 mysql-server/root_password_again password development_pass" | sudo debconf-set-selections
    DEBIAN_FRONTEND="noninteractive" sudo apt-get install mysql-server-5.7 -y
fi

# check binary
echo "[CoIn]: Building SCION Coordinator binary..."
rm -f "scion-coord"
pushd "$SCIONCOORD" >/dev/null
go build
if [ ! -x "scion-coord" ]; then
    echo "[CoIn]: Still didn't find the binary. Abort."
    exit 1
fi
popd >/dev/null
echo "[CoIn]: done (SCION Coordinator binary built)."

if [ "$inside_docker" == 1 ]; then
    echo "[CoIn]: Running in docker. Configuring MySQL to run on host 'mysql'."
    # copy the default configuration and edit it for postmark
    cp "$SCIONCOORD/conf/development.conf.default" "$SCIONCOORD/conf/development.conf"
    sed -i -- 's/email.pm_server_token = ""/email.pm_server_token = "server_token"/g' "$SCIONCOORD/conf/development.conf"
    sed -i -- 's/email.pm_account_token = ""/email.pm_account_token = "account_token"/g' "$SCIONCOORD/conf/development.conf"
    echo "[CoIn]: Success."
    exit 0
fi

############## NON DOCKER system:

# VPN stuff
mkdir -p "$SCIONCOORD/credentials"
if [ ! -f "$CONFDIR/easy-rsa/keys/ca.crt" ]; then
    # generate certificate for openVPN
    echo "[CoIn]: Certificate for OpenVPN not found. Manually copy it from the backup to $CONFDIR/easy-rsa/keys/"
    mkdir -p "$CONFDIR/easy-rsa/keys"
    exit 1
fi

# check if mysqld is running:
if ! pgrep -x "mysqld" &>/dev/null; then
    echo "[CoIn]: MySQL is not running. Starting the service."
    sudo systemctl restart mysql
fi

if ! runSQL "SHOW DATABASES;" | grep "scion_coord_test" &> /dev/null; then
    runSQL "CREATE DATABASE scion_coord_test;" || (echo "Failed to create the SCION Coordinator DB" && exit 1)
    echo "[CoIn]: Empty DB created"
fi

# check service file
if [ ! -f "$files/scion-coord.service" ] \
   || [ ! -f "$files/unit-status-mail@.service" ] \
   || [ ! -f "$files/emailer.py" ]; then
    echo "[CoIn]: Missing service file"
    exit 1
fi
if [ ! -d "/etc/systemd/system" ]; then
    echo "[CoIn]: Cannot find the destination directory for the service file. /etc/systemd/system not found. Abort."
    exit 1
fi

sudo systemctl stop "scion-coord" || true
pushd "/etc/systemd/system" >/dev/null
if [ -f "scion-coord.service" ]; then
    echo "[CoIn]: There was a previous scion-coord.service file. Replaced it."
    sudo rm -f scion-coord.service
fi
tmpfile=$(mktemp)
cp "$files/scion-coord.service" "$tmpfile"
sed -i -- "s/_USER_/$USER/g" "$tmpfile"
sudo cp "$tmpfile" "scion-coord.service"
cp "$files/unit-status-mail@.service" "$tmpfile"
sed -i -- "s/_USER_/$USER/g" "$tmpfile"
sudo cp "$tmpfile" "unit-status-mail@.service"
popd >/dev/null
sudo cp "$files/emailer.py" "/usr/local/bin/emailer"

# if it doesn't exist, create the default configuration for the emailer:
if [ ! -f "$HOME/.config/scion-coord/email.conf" ] || [ ! -f "$HOME/.config/scion-coord/recipients.txt" ][ ! -f "$HOME/.config/scion-coord/recipients.txt" ] then
    echo "[CoIn]: We need the email.conf file. Please read the $files/README.md file"
    exit 1
fi

# submodules
pushd "$basedir" >/dev/null
git submodule update --remote --recursive
# in particular for the update V2, sub/scion_nextversion needs to be updated with one file, by running make:
cd "sub/scion_nextversion/go"
make ../proto/go.capnp
popd >/dev/null

sudo systemctl daemon-reload
sudo systemctl start "scion-coord"
sleep 1
systemctl status "scion-coord" >/dev/null && fail=0 || fail=1
if [ $fail -ne 0 ]; then
    echo "[CoIn]: Problem starting the service. Please check with systemctl status scion-coord or journalctl -xe"
    exit 1
fi
sudo systemctl enable "scion-coord"
echo "[CoIn]: Success."
