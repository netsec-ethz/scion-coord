#!/bin/bash

# Configure a coordinator service and test SCION works with it

set -e
# set -x

# check and export paths:
if [ -z "$GOPATH" ]; then
    GOPATH="$HOME/go"
fi

MYSQLCMD="mysql -u root -pdevelopment_pass"
NETSEC=${GOPATH:?}/src/github.com/netsec-ethz
SCIONCOORD="$NETSEC/scion-coord"
SCION="${GOPATH:?}/src/github.com/scionproto/scion"
CONFDIR="$HOME/scionLabConfigs"
EASYRSADEFAULT="$SCIONCOORD/conf/easy-rsa_vars.default"
TESTTIMEOUT=8

ACC_ID="someid"
ACC_PW="some_secret"
INTF_ADDR="127.0.0.5"
SCION_COORD_URL="http://localhost:8080"

missingOrDifferentFiles() {
    ! [[ -f "$1" ]] || ! [[ -f "$2" ]] || ! cmp "$1" "$2" >/dev/null
}

runSQL() {
    SQL=$1
    output=$(echo $SQL | $MYSQLCMD 2>&1) ; FAIL=$?
    output=$(echo "$output" | grep -Fv [Warning])
    echo "$output"
    return $FAIL
}

onExit() {
    RET=$?
    trap '' INT TERM
    if [ "$doTest" -eq 1 ]; then
        # this means that we have stopped and started the scion ourselves. Stop it back
        cd "$SCION"
        "./scion.sh" stop || true
    fi
    if [ ! -z $scionCoordPid ]; then
        # maybe kill SCION Coord if it's running and wait
        kill -TERM 0
        wait
    fi
    if [ ! -z "$restoreGen" ]; then
        rm -rf "$SCION/gen-previous-coordinator-test-run"
        mv "$SCION/gen" "$SCION/gen-previous-coordinator-test-run"
        mv "$restoreGen" "$SCION/gen"
    fi
    [[ ! -z "$exitMessage" ]] && printf "$exitMessage""\n"
    exit $RET
}
trap onExit EXIT INT TERM

cleanZookeeper() {
    if [ -x /usr/share/zookeeper/bin/zkCli.sh ] && [ /usr/share/zookeeper/bin/zkServer.sh status &>/dev/null ]; then
        printf 'rmr /1-11 \nrmr /1-12 \nrmr /1-13 \nrmr /1-1001 \n' | /usr/share/zookeeper/bin/zkCli.sh &>/dev/null
    fi
}

archiveGenFolder() {
    if [ -d "$SCION/gen" ]; then
        restoreGen=$(mktemp -d)
        rmdir "$restoreGen"
        mv "$SCION/gen" "$restoreGen"
    fi
}

scionCoordPid=''
exitMessage=''
restoreGen=''
CURRENTWD="$PWD"
thisdir="$(dirname $(realpath $0))"
mkdir -p "$NETSEC"
cd "$NETSEC"

doTest=1
usage="$(basename $0) [-n]

where:
    -n      No test: only set up SCION + coordinator, run coordinator and wait for it to finish"
while getopts ":n" opt; do
case $opt in
    h)
        echo "$usage"
        exit 0
        ;;
    n)
        doTest=0
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

if [ ! -f "$thisdir/scion_install_script.sh" ]; then
    exitMessage="Could not find the SCION installation script. Aborting."
    exit 1
fi
cleanZookeeper
archiveGenFolder
bash "$thisdir/scion_install_script.sh"
source ~/.profile

# check requirements to be an attachment point
if [ "$doTest" -eq 1 ]; then
    echo "Installing AP for testing..."
    cd "$CURRENTWD"
    if [ ! -d "${SCIONBOXLOCATION:?}" ]; then
        echo "Error: ${SCIONBOXLOCATION:?} doesn't look like a valid directory for scion-box"
        exit 1
    fi
    pushd "${SCIONBOXLOCATION:?}" &>/dev/null
    ./scripts/install_attachment_point.sh -t
    popd &>/dev/null
    echo "Done installing AP for testing."
fi

# check go dependencies
command -v govendor >/dev/null 2>&1 || go get github.com/kardianos/govendor
cd "$SCIONCOORD/vendor"
govendor sync

# check other requirements to be a Coordinator:
if ! python3 -c "from Crypto import Random" &>/dev/null; then
    echo "Installing required packages..."
    pip3 install --upgrade pycrypto
    echo "Done installing required packages."
fi

# Custom configuration
if [ ! -f "$SCIONCOORD/conf/development.conf" ]; then
    # copy the default configuration and edit it for postmark
    cp "$SCIONCOORD/conf/development.conf.default" "$SCIONCOORD/conf/development.conf"
    sed -i -- 's/email.pm_server_token = ""/email.pm_server_token = "server_token"/g' "$SCIONCOORD/conf/development.conf"
    sed -i -- 's/email.pm_account_token = ""/email.pm_account_token = "account_token"/g' "$SCIONCOORD/conf/development.conf"
fi

# MySQL DB step. If already installed, we assume it's already configured the way we want
if ! dpkg-query -s mysql-server &> /dev/null ; then
    echo "mysql-server-5.7 mysql-server/root_password password development_pass" | sudo debconf-set-selections
    echo "mysql-server-5.7 mysql-server/root_password_again password development_pass" | sudo debconf-set-selections
    DEBIAN_FRONTEND="noninteractive" sudo apt-get install mysql-server -y
fi
# check if mysqld is running:
if ! pgrep -x "mysqld" &>/dev/null; then
    echo "MySQL is not running. Starting the service."
    sudo systemctl restart mysql
fi

if ! dpkg-query -s easy-rsa &> /dev/null ; then
    DEBIAN_FRONTEND="noninteractive" sudo apt-get install easy-rsa -y
fi

if ! runSQL "SHOW DATABASES;" | grep "scion_coord_test" &> /dev/null; then
    runSQL "CREATE DATABASE scion_coord_test;" || (echo "Failed to create the SCION Coordinator DB" && exit 1)
else
    echo "Removing entries in the DB"
    # we need the id > 0 to convince mysql that it is secure
    runSQL "DELETE FROM scion_coord_test.connection WHERE id > 0 AND respond_ap IN (
    SELECT id FROM scion_coord_test.attachment_point WHERE as_ID IN (
        SELECT id from scion_coord_test.scion_lab_as WHERE user_email='netsec.test.email@gmail.com'
        )
    );" &>/dev/null || true
    runSQL "DELETE FROM scion_coord_test.attachment_point WHERE id > 0 AND as_id IN (
    SELECT id FROM scion_coord_test.scion_lab_as WHERE user_email='netsec.test.email@gmail.com'
    );" &>/dev/null || true
    runSQL "DELETE FROM scion_coord_test.scion_lab_as WHERE id > 0 AND user_email='netsec.test.email@gmail.com';" &>/dev/null || true
    runSQL "DELETE FROM scion_coord_test.user WHERE id > 0 AND email='netsec.test.email@gmail.com';" &>/dev/null || true
    runSQL "DELETE FROM scion_coord_test.account WHERE id > 0 AND name='netsec.test.email@gmail.com';" &>/dev/null || true
fi

# now copy the three credentials files from the SCION installation to the coordinator
# we should do this per ISD with an attachment point
mkdir -p "$SCIONCOORD/credentials"
cd "$SCIONCOORD/credentials"
VERS=$(ls $SCION/gen/ISD1/AS11/br1-11-1/certs/*.crt | sort -r | awk 'NR==1{print $1}' | sed -n 's/^.*ISD1-AS11-V\([0-9]*\)\.crt/\1/p')
if missingOrDifferentFiles "$SCION/gen/ISD1/AS11/br1-11-1/keys/as-sig.seed" ISD1.key ||
   missingOrDifferentFiles "$SCION/gen/ISD1/AS11/br1-11-1/certs/ISD1-AS11-V$VERS.crt" ISD1.crt ||
   missingOrDifferentFiles "$SCION/gen/ISD1/AS11/br1-11-1/certs/ISD1-V$VERS.trc" ISD1.trc ;
then
    echo "Credentials in SCION Coord. Serv. seem different. Running SCION and using those"
    pushd "$SCION" >/dev/null
    ./scion.sh topology -c topology/Tiny.topo
    popd >/dev/null

    cp "$SCION/gen/ISD1/AS11/br1-11-1/keys/as-sig.seed" ISD1.key
    VERS=$(ls $SCION/gen/ISD1/AS11/br1-11-1/certs/*.crt | sort -r | awk 'NR==1{print $1}' | sed -n 's/^.*ISD1-AS11-V\([0-9]*\)\.crt/\1/p')
    cp "$SCION/gen/ISD1/AS11/br1-11-1/certs/ISD1-AS11-V$VERS.crt" ISD1.crt
    cp "$SCION/gen/ISD1/AS11/br1-11-1/certs/ISD1-V$VERS.trc" ISD1.trc
fi

# VPN stuff
if [ ! -f "$CONFDIR/easy-rsa/keys/ca.crt" ]; then
    # generate certificate for openVPN
    if ! dpkg-query -s easy-rsa &> /dev/null ; then
        DEBIAN_FRONTEND="noninteractive" sudo apt-get install easy-rsa -y
    fi
    if ! dpkg-query -s openssl &> /dev/null ; then
        DEBIAN_FRONTEND="noninteractive" sudo apt-get install openssl -y
    fi
    mkdir -p "$CONFDIR"
    cp -r /usr/share/easy-rsa "$CONFDIR"
    cp "$EASYRSADEFAULT" "$CONFDIR/easy-rsa/vars"
    pushd "$CONFDIR/easy-rsa" >/dev/null
    sed -i -- 's/export KEY_EMAIL="scion@lists.inf.ethz.ch"/export KEY_EMAIL="netsec.test.email@gmail.com"/g' ./vars
    source ./vars
    ./clean-all
    # build the CA non interactively
    ./pkitool --initca
    popd >/dev/null
fi

# build and run:
cd "$SCIONCOORD"
go build
./scion-coord --help &> /dev/null

# populate the SCION coord. test DB accordingly. For now with one attachment point, in ISD1 AS12
sql="SELECT COUNT(*) FROM scion_coord_test.account WHERE name='netsec.test.email@gmail.com';"
out=$(runSQL "$sql") && stat=0 || stat=$?
out=$(echo "$out" | tail -n 1)
if [ $out -ne 0 ]; then
    exitMessage="Inconsistent result: we deleted all data related to the test in the DB, but still have an account. Aborting."
    exit 1
fi

sql="INSERT INTO scion_coord_test.account
(id, name, organisation, account_id, secret, created, updated)
VALUES
(1, 'netsec.test.email@gmail.com', 'NETSEC TEST', '$ACC_ID', '$ACC_PW', NOW(), NOW())
"
out=$(runSQL "$sql") && stat=0 || stat=$?

# password is "scionscion"
sql="INSERT INTO scion_coord_test.user
(id, email, password, password_invalid, salt,
first_name, last_name, verified, is_admin, verification_uuid, account_id, created, updated)
VALUES
(1, 'netsec.test.email@gmail.com', '81c0cd129972d7f5ebda612da8c13528e80068705330170121d9b07bdc52b7f0', 0, '286301951c5da8c82dd34f6123ce05ef17fc0f0c1032067eca4a909c0f0f03e85c0123f3c8510afec0809aebfb74dafad300c4a847c787e34628a2bb5c336e94705ab076c9103452064ce448be2a416c',
'first name', 'last name', 1, 0, '0371c50c-511d-417f-bbee-949df9fe52c6',
1, NOW(), NOW()
)"
out=$(runSQL "$sql") && stat=0 || stat=$?

sql="INSERT INTO scion_coord_test.scion_lab_as
(id, user_email,                   public_ip,   start_port, label,  isd, as_id, status, type,  created, updated)
VALUES
(2, 'netsec.test.email@gmail.com', '$INTF_ADDR', 50000,      'AS12', 1,     12,      1,      0, now(),   now());"
out=$(runSQL "$sql") && stat=0 || stat=$?

sql="INSERT INTO scion_coord_test.attachment_point
(vpn_ip, start_vpn_ip, end_vpn_ip, as_id)
SELECT '10.0.8.1', '10.0.8.2',  '10.0.8.254', id
FROM scion_coord_test.scion_lab_as WHERE user_email='netsec.test.email@gmail.com';"
out=$(runSQL "$sql") && stat=0 || stat=$?

# remove already generated configuration TGZs :
rm -rf "$CONFDIR/netsec.test.email*"

# run SCION Coordinator
./scion-coord &
scionCoordPid=$!

# wait until the HTTP service is up, or 5 seconds
timeout 5 bash -c "until curl --output /dev/null --silent --head --fail $SCION_COORD_URL; do
    echo 'Waiting for SCION Coord. Service to be up ...'
    sleep 1
done"

if [ "$doTest" -ne 1 ]; then
    # only run the coordinator and wait until it finishes
    echo "The Coordinator is running with PID: $scionCoordPid"
    wait $scionCoordPid
    exit $?
fi

# TEST SCION COORDINATOR. The requests don't need to have all these headers, but hey were just copied from Chrome for convenience
echo "Querying SCION Coordinator Service to create an AS, configure it and download its gen folder definition..."
rm -f cookies.txt
curl "$SCION_COORD_URL" -I -c cookies.txt -s >/dev/null
curl "$SCION_COORD_URL/api/login" -H 'Content-Type: application/json;charset=UTF-8' -b cookies.txt --data-binary '{"email":"netsec.test.email@gmail.com","password":"scionscion"}' --compressed -s >/dev/null
curl "$SCION_COORD_URL/api/as/generateAS" -X POST -H 'Content-Length: 0' -b cookies.txt -s >/dev/null
curl "$SCION_COORD_URL/api/as/configureAS" -H 'Content-Type: application/json;charset=UTF-8' -b cookies.txt --data-binary '{"asID":1001,"userEmail":"netsec.test.email@gmail.com","isVPN":false,"ip":"127.0.0.210","serverIA":"1-12","label":"Label for AS1001","type":2,"port":50050}' -s >/dev/null
GENFOLDERTMP=$(mktemp -d)
rm -rf "$GENFOLDERTMP"
mkdir -p "$GENFOLDERTMP"
curl "$SCION_COORD_URL/api/as/downloadTarball/1001" -b cookies.txt --output "$GENFOLDERTMP/1001.tgz" -s >/dev/null
rm -f cookies.txt

if [ ! -f "$GENFOLDERTMP/1001.tgz" ]; then
    exitMessage="Cannot find the (presumably) downloaded file $GENFOLDERTMP/1001.tgz\nFAIL"
    exit 101
fi
cd "$GENFOLDERTMP"
tar xf "1001.tgz"
if [ ! -d "netsec.test.email@gmail.com_1-1001/gen/ISD1/AS1001" ]; then
    exitMessage="Unknown TGZ structure. Cannot continue\nABORT"
    exit 1
fi
# safety check:
if [ -d "$SCION/gen/ISD1/AS1001" ]; then
    pushd "$SCION/gen/ISD1" >/dev/null
    mv "AS1001" "AS1001-renamed-testing-scion-coord-$(date +%Y.%m.%dT%H:%M:%S.%N)"
    popd >/dev/null
fi
cp -r "netsec.test.email@gmail.com_1-1001/gen/ISD1/AS1001" "$SCION/gen/ISD1/"
cd "$SCION"
rm -rf "$GENFOLDERTMP"

# update existing AS12 using the scion box update-gen script
pushd "$CURRENTWD" >/dev/null
# run update gen:
cd "${SCIONBOXLOCATION:?}"
torun="./updateGen.sh"
params="--url $SCION_COORD_URL --address $INTF_ADDR --accountId $ACC_ID --secret $ACC_PW --updateAS 1-12"
echo "Calling: $torun $params"
"$torun" "$params"
popd >/dev/null

# we are done using SCION Coord; shut it down
kill $scionCoordPid
scionCoordPid=''
echo "SCION Coordinator service was stopped."

echo "Running SCION now:"
cd "$SCION"
"./scion.sh" stop || true
"./scion.sh" run

echo "Checking logs for successful arrival of beacons to the new AS (or $TESTTIMEOUT seconds)..."
FOUND=false

exec 4>&2
exec 2>/dev/null
# run timeout tail in a subshell because we need the while read loop in this one, to set FOUND to true iff found
exec 3< <(timeout $TESTTIMEOUT tail -n0 -f "$SCION/logs/bs1-1001-1.DEBUG")
exec 2>&4 4>&-
SSPID=$!
while read -u 3 LINE; do
    if echo $LINE | grep 'Successfully verified PCB' &> /dev/null; then
        FOUND=true
        pkill -P $SSPID "timeout" &> /dev/null || true
    fi
done

if [[ "$FOUND" = true ]]; then
    exitMessage="SUCCESS"
    exit 0
else
    exitMessage="FAIL"
    exit 100
fi
