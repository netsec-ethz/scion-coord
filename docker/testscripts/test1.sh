#!/bin/bash

# Configure a coordinator service and test SCION works with it
sleep 20
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
TESTTIMEOUT=20

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

# Zookeeper won't run in this container, redirect its port:
redir --lport=2181 --caddr=zookeeper1 --cport=2181 &


# wait until mysql is ready:
echo "Waiting for MySql server ..."
while ! mysqladmin ping -h mysql --silent; do
    sleep 1
done
echo "MySql server found."
runSQL "DROP DATABASE scion_coord_test;" &>/dev/null || true
runSQL "CREATE DATABASE scion_coord_test;" || (echo "Failed to create the SCION Coordinator DB" && exit 1)

# create the initial tables:
cd "$SCIONCOORD"
rm -f scion-coord
# Stop previous run
scioncoord_pid="$(ps aux | grep "\./scion-coord" | grep -v grep | awk '{print $2}')"
if [[ ${scioncoord_pid} != "" ]]; then
    echo "./scion-coord already running at PID ${scioncoord_pid}, killing PID ${scioncoord_pid}"
    kill ${scioncoord_pid}
fi
# Build coordinator and check it is runnable
go build
./scion-coord --help >/dev/null

# populate the test DB accordingly. For now with one attachment point, in ISD1 ASffaa:0:111
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
(id, user_email,                    public_ip,   start_port, label,        isd, as_id,             status, type,  created, updated)
VALUES
(2, 'netsec.test.email@gmail.com', '$INTF_ADDR', 50000,      'old AS12',   1,   0xff0000000111,    1,      0,     now(),   now());"
echo "Inserting AS for user 'netsec.test.email@gmail.com'"
echo "echo $sql | $MYSQLCMD 2>&1"
out=$(runSQL "$sql") && stat=0 || stat=$?
echo $out

sql="INSERT INTO scion_coord_test.attachment_point
(vpn_ip, start_vpn_ip, end_vpn_ip, as_id)
SELECT '10.0.8.1', '10.0.8.2',  '10.0.8.254', id
FROM scion_coord_test.scion_lab_as WHERE user_email='netsec.test.email@gmail.com';"
out=$(runSQL "$sql") && stat=0 || stat=$?

# run SCION Coordinator in the background:
./scion-coord 2>&1 &
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
curl "$SCION_COORD_URL/api/as/configureAS" -H 'Content-Type: application/json;charset=UTF-8' -b cookies.txt --data-binary '{"asID":281105609588737,"userEmail":"netsec.test.email@gmail.com","isVPN":false,"ip":"127.0.0.210","serverIA":"1-ff00:0:111","label":"Label for ASffaa:1:1 (old AS1001)","type":2,"port":50050}' -s >/dev/null
GENFOLDERTMP=$(mktemp -d)
rm -rf "$GENFOLDERTMP"
mkdir -p "$GENFOLDERTMP"
curl "$SCION_COORD_URL/api/as/downloadTarball/ffaa_1_1" -b cookies.txt --output "$GENFOLDERTMP/ffaa_1_1.tgz" -s >/dev/null
rm -f cookies.txt

if [ ! -f "$GENFOLDERTMP/ffaa_1_1.tgz" ]; then
    echo -e "Cannot find the (presumably) downloaded file $GENFOLDERTMP/ffaa_1_1.tgz\nFAIL"
    exit 101
fi
if [ $(file "$GENFOLDERTMP/ffaa_1_1.tgz" | awk '{print $2}') != "gzip" ]; then
    echo -e "The downloaded file $GENFOLDERTMP/ffaa_1_1.tgz is not a valid gzip file.\nFAIL"
    exit 102
fi
cd "$GENFOLDERTMP"

echo "gunzipping"
gunzip "ffaa_1_1.tgz"

echo "Untaring"
tar xf "ffaa_1_1.tar"
if [ ! -d "netsec.test.email@gmail.com_1-ffaa_1_1/gen/ISD1/ASffaa_1_1" ]; then
    echo -e "Unknown TGZ structure. Cannot continue\nABORT"
    exit 1
fi
rm -rf "$SC/gen/ISD1/ASffaa_1_1"
cp -r "netsec.test.email@gmail.com_1-ffaa_1_1/gen/ISD1/ASffaa_1_1" "$SC/gen/ISD1/"
rm -rf "$GENFOLDERTMP"

# update existing ASffaa:0:111 using the scion box update-gen script
cd "$SC"
pushd "${SCIONBOXLOCATION:?}" >/dev/null
torun="${SCIONBOXLOCATION:?}/updateGen.sh"
params="--url $SCION_COORD_URL --address $INTF_ADDR --accountId $ACC_ID --secret $ACC_PW --updateAS 1-ff00_0_111"
echo "Calling: $torun $params"
"$torun" "$params" &>/dev/null
popd >/dev/null



# TODO: remove after https://github.com/netsec-ethz/scion-utilities/issues/17 is fixed :
# patch: replace the wrongfully assigned default.sock socket name with the right one sd1-ff00_0_111.sock :
find "$SC/gen/ISD1/ASff00_0_111/" -type f -name 'supervisord.conf' -exec sed -i 's/default.sock/sd1-ff00_0_111.sock/' "{}" +
echo "Replaced default.sock with the correct one sd1-ff00_0_111.sock . Please remove this patch after issue #17 in scion-utilities has been fixed"
./scion.sh stop || true
./supervisor/supervisor.sh reload
# end of patch



# we are done using SCION Coord; shut it down
kill $scionCoordPid
echo "SCION Coordinator service was stopped."

echo "Running SCION now:"
cd "$SC"
./scion.sh stop || true
./scion.sh start

echo "Checking logs for successful arrival of beacons to the new AS (or $TESTTIMEOUT seconds)..."
FOUND=false
exec 4>&2
exec 2>/dev/null
# run timeout tail in a subshell because we need the while read loop in this one, to set FOUND to true iff found
exec 3< <(timeout $TESTTIMEOUT tail -n0 -f "$SC/logs/bs1-ffaa_1_1-1.DEBUG")
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
