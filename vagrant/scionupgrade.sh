#!/bin/bash

#TODO: Load client ID and client secret

#TODO: replace URL with correct address
wget https://gist.githubusercontent.com/xabarass/def1f861f0fbb1f51d7479d23e239c6f/raw/1de4daf4594abd28d383a0a0d1d0110e9398c670/upgrade.sh
chmod +x upgrade.sh

./upgrade.sh

OUT=$?
if [ $OUT -eq 0 ];then
   echo "Update script executed without errors."
   #TODO: Report success
else
   echo "Update script exited with error code!"
   #TODO: Report error
fi
