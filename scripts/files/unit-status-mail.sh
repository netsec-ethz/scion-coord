#!/bin/bash
MAILTO="scionlab-team@lists.inf.ethz.ch"
MAILFROM="unit-status-mailer"
UNIT=$1

EXTRA=""
for e in "${@:2}"; do
  EXTRA+="$e"$'\n'
done

UNITSTATUS=$(systemctl status $UNIT)

sendmail $MAILTO <<EOF
From:$MAILFROM
To:$MAILTO
Subject:Status mail for unit: $UNIT

Status report for unit: $UNIT
$EXTRA

$UNITSTATUS
EOF

echo -e "Status mail sent to: $MAILTO for unit: $UNIT"
