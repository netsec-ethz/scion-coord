#!/bin/bash

# Configure a coordinator service and test SCION works with it

set -e

# check and export paths:
if [ -z "$GOPATH" ]; then
    GOPATH="$HOME/go"
fi

MYSQLCMD="mysql -u root -pdevelopment_pass -h mysql"
NETSEC=${GOPATH:?}/src/github.com/netsec-ethz
SCIONCOORD="$NETSEC/scion-coord"
SCIONBOXLOCATION="$SCIONCOORD/sub/scion-box"
CONFDIR="$HOME/scionLabConfigs"
EASYRSADEFAULT="$SCIONCOORD/conf/easy-rsa_vars.default"
TESTTIMEOUT=8

ACC_ID="someid"
ACC_PW="some_secret"
INTF_ADDR="127.0.0.5"
SCION_COORD_URL="http://localhost:8080"


runSQL() {
    SQL=$1
    output=$(echo $SQL | $MYSQLCMD 2>&1) ; FAIL=$?
    output=$(echo "$output" | grep -Fv [Warning])
    echo "$output"
    return $FAIL
}


CURRENTWD="$PWD"
basedir="$(realpath $(dirname $(realpath $0))/../)"

mkdir -p "$NETSEC"
cd "$NETSEC"


# wait until mysql is ready:
while ! mysqladmin ping -h mysql --silent; do
    sleep 1
done
runSQL "DROP DATABASE scion_coord_test;" &>/dev/null || true
runSQL "CREATE DATABASE scion_coord_test;" || (echo "Failed to create the SCION Coordinator DB" && exit 1)


# create the initial tables:
cd "$SCIONCOORD"
rm -f scion-coord
go build
./scion-coord --help >/dev/null

# populate the test DB accordingly. For now with one attachment point, in ISD1 AS12
sql="SELECT COUNT(*) FROM scion_coord_test.account WHERE name='netsec.test.email@gmail.com';"
out=$(runSQL "$sql") && stat=0 || stat=$?
out=$(echo "$out" | tail -n 1)
if [ $out -ne 0 ]; then
    echo "Inconsistent result: we deleted all data related to the test in the DB, but still have an account. Aborting."
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


# run SCION Coordinator in the background:
./scion-coord &
scionCoordPid=$!

# wait until the Coordinator HTTP service is up, or 5 seconds
timeout 5 bash -c "until curl --output /dev/null --silent --head --fail $SCION_COORD_URL; do
    echo 'Waiting for SCION Coord. Service to be up ...'
    sleep 1
done"


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
    echo "Cannot find the (presumably) downloaded file $GENFOLDERTMP/1001.tgz\nFAIL"
    exit 101
fi
cd "$GENFOLDERTMP"
tar xf "1001.tgz"
if [ ! -d "netsec.test.email@gmail.com_1-1001/gen/ISD1/AS1001" ]; then
    echo "Unknown TGZ structure. Cannot continue\nABORT"
    exit 1
fi
rm -rf "$SC/gen/ISD1/AS1001"
cp -r "netsec.test.email@gmail.com_1-1001/gen/ISD1/AS1001" "$SC/gen/ISD1/"
rm -rf "$GENFOLDERTMP"
pwd 
cd "$SC"
pwd

# update existing AS12 using the scion box update-gen script
pushd "$CURRENTWD" >/dev/null
pwd 
# run update gen:
cd "${SCIONBOXLOCATION:?}"
pwd
if [ $(ls sub/util | wc -l) == 0 ]; then
    git submodule init
    git submodule update
fi
torun="./updateGen.sh"
params="--url $SCION_COORD_URL --address $INTF_ADDR --accountId $ACC_ID --secret $ACC_PW --updateAS 1-12"
echo "Calling: $torun $params"
"$torun" "$params" &>/dev/null
popd >/dev/null

# we are done using SCION Coord; shut it down
kill $scionCoordPid
echo "SCION Coordinator service was stopped."

echo "Running SCION now:"
cd "$SC"
"./scion.sh" stop || true
"./scion.sh" run

echo "Checking logs for successful arrival of beacons to the new AS (or $TESTTIMEOUT seconds)..."
FOUND=false

exec 4>&2
exec 2>/dev/null
# run timeout tail in a subshell because we need the while read loop in this one, to set FOUND to true iff found
exec 3< <(timeout $TESTTIMEOUT tail -n0 -f "$SC/logs/bs1-1001-1.DEBUG")
exec 2>&4 4>&-
SSPID=$!
while read -u 3 LINE; do
    if echo $LINE | grep 'Successfully verified PCB' &> /dev/null; then
        FOUND=true
        pkill -P $SSPID "timeout" &> /dev/null || true
    fi
done

if [[ "$FOUND" = true ]]; then
    echo "SUCCESS"
    exit 0
else
    echo "FAIL"
    exit 100
fi
