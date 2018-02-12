#!/bin/bash

set -e

areAllLinesInsideFile() {
    lines=$1
    filename="$2"
    for line in $lines; do
        grep "$line" "$filename" &> /dev/null || return 1
    done
    return 0
}


shopt -s nullglob

usage="$(basename "$0") [-p PATCH_DIR] [-g GEN_DIR] [-v VPN_CONF_PATH] \
[-s SCION_SERVICE] [-z SCION_VI_SERVICE] [-a ALIASES_FILE] [-c]

where:
    -p PATCH_DIR        apply patches from PATCH_DIR on cloned repo
    -g GEN_DIR          path to gen directory to be used
    -v VPN_CONF_PATH    path to OpenVPN configuration file
    -s SCION_SERVICE    path to SCION service file
    -z SCION_VI_SERVICE path to SCION-viz service file
    -a ALIASES_FILE     adds useful command aliases in specified file
    -c                  do not destroy user context on logout"

while getopts ":p:g:v:s:z:ha:c" opt; do
  case $opt in
    p)
      patch_dir=$OPTARG
      ;;
    g)
      gen_dir=$OPTARG
      ;;
    v)
      vpn_config_file=$OPTARG
      ;;
    s)
      scion_service_path=$OPTARG
      ;;
    z)
      scion_viz_service=$OPTARG
      ;;
    h)
      echo "Displaying help:" >&2
      echo "$usage" >&2
      exit 1
      ;;
    a)
      aliases_file=$OPTARG
      ;;
    c)
      keep_user_context=true
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

echo "Starting SCION installation..."

# Check if we are running on correct Ubuntu system
if [ -f /etc/os-release ]
then
    . /etc/os-release

    if [[ $NAME == "Ubuntu" && $VERSION_ID == 16.04* ]] ; then
      echo "We are running on $NAME version $VERSION_ID seems okay"
    else
      echo "ERROR! We are not running on Ubuntu 16.04 system, shutting down!" >&2
      exit 1
    fi
else
    echo "ERROR! This script can only be run on Ubuntu 16.04" >&2
    exit 1
fi

command -v git >/dev/null 2>&1 || sudo apt-get -y install git

source ~/.profile
echo "$GOPATH" | grep "$HOME/go" &> /dev/null || echo 'export GOPATH="$HOME/go"' >> ~/.profile
echo "$PATH" | grep "/usr/local/go/bin" &> /dev/null || echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.profile
echo "$PATH" | grep "$GOPATH/bin" &> /dev/null || echo 'export PATH="$GOPATH/bin:$PATH"' >> ~/.profile
echo "$PATH" | grep "$HOME/.local/bin" &> /dev/null || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.profile
echo "$SC" | grep "$GOPATH/src/github.com/netsec-ethz/scion" &> /dev/null || echo 'export SC="$GOPATH/src/github.com/netsec-ethz/scion"' >> ~/.profile
echo $PYTHONPATH | grep "$SC/python" &> /dev/null || echo 'export PYTHONPATH="$SC/python:$SC"' >> ~/.profile

source ~/.profile
mkdir -p "$GOPATH"
mkdir -p "$GOPATH/src/github.com/netsec-ethz"
cd "$GOPATH/src/github.com/netsec-ethz"

git config --global url.https://github.com/.insteadOf git@github.com:
if [ ! -d scion ]
then
    git clone --recursive -b scionlab git@github.com:netsec-ethz/netsec-scion scion
fi

cd scion

# Check if there is a patch directory
if  [[ ( ! -z ${patch_dir+x} ) && -d ${patch_dir} ]]
then
    echo "Applying patches:"
    patch_files="$patch_dir/*.patch"

    for f in $patch_files;
    do
        echo -e "\t$f"
        git apply "$f"
    done

    git_username=$(git config user.name || true)

    # We need to have git user in order to commit
    if [ -z "$git_username" ]
    then
        echo "GIT user credentials not set, configuring defaults"
        git config --global user.name "Scion User" 
        git config --global user.email "scion@scion-architecture.net"
    fi

    git commit -am "Applied platform dependent patches"

    echo "Finished applying patches"
fi

echo "Installing dependencies..."
bash -c 'yes | GO_INSTALL=true ./env/deps' > /dev/null


if ! areAllLinesInsideFile "$(cat docker/zoo.cfg)" "/etc/zookeeper/conf/zoo.cfg";
then
    sudo cp docker/zoo.cfg /etc/zookeeper/conf/zoo.cfg
fi

# Add cron script which removes old zk logs
if [ ! -f "/etc/cron.daily/zookeeper" ]
then
    sudo bash -c 'cat > /etc/cron.daily/zookeeper << CRON1
#! /bin/sh
/usr/share/zookeeper/bin/zkCleanup.sh -n 3
CRON1'
    sudo chmod 755 /etc/cron.daily/zookeeper
fi

# Check if gen directory exists
if  [[ ( ! -z ${gen_dir+x} ) && -d ${gen_dir} ]]
then
    echo "Gen directory is specified! Using content from there!"
    cp -r "$gen_dir" .
elif [ ! -d gen ]; then
    echo "Gen directory is NOT specified! Generating local (Tiny) topology!"
    ./scion.sh topology -c topology/Tiny.topo
fi

cd sub
if [ ! -d "scion-viz" ]; then
    git clone git@github.com:netsec-ethz/scion-viz
    pushd scion-viz/python/web >/dev/null
    pip3 install --user --require-hashes -r requirements.txt
    python3 ./manage.py migrate
    popd >/dev/null
fi

# Should we add aliases
if [[ (! -z ${aliases_file} ) ]]
then
  echo "Adding aliases to $aliases_file"

  echo "alias cdscion='cd $SC'" >> "$aliases_file"
  echo "alias checkbeacons='tail -f $SC/logs/bs*.DEBUG'" >> "$aliases_file"
fi

if  [[ ( ! -z ${vpn_config_file+x} ) && -r ${vpn_config_file} ]]
then
    echo "VPN configuration specified! Configuring it!"

    sudo apt-get -y install openvpn

    sudo cp "$vpn_config_file" /etc/openvpn/client.conf
    sudo chmod 600 /etc/openvpn/client.conf
    sudo systemctl start openvpn@client
    sudo systemctl enable openvpn@client
fi

if  [[ ( ! -z ${scion_service_path+x} ) && -r ${scion_service_path} ]]
then
    echo "Registering SCION as startup service"

    cp "$scion_service_path" tmp.service
    # We need to replace template user with current username
    sed -i "s/_USER_/$USER/g" tmp.service
    sudo cp tmp.service /etc/systemd/system/scion.service

    sudo systemctl enable scion.service
    sudo systemctl start scion.service

    rm tmp.service
else
    echo "SCION systemd service file not specified! SCION won't run automatically on startup."
fi

if  [[ ( ! -z ${scion_viz_service+x} ) && -r ${scion_viz_service} ]]
then
    echo "Registering SCION-viz as startup service"

    cp "$scion_viz_service" tmp.service
    # We need to replace template user with current username
    sed -i "s/_USER_/$USER/g" tmp.service
    sudo cp tmp.service /etc/systemd/system/scion-viz.service

    sudo systemctl enable scion-viz.service
    sudo systemctl start scion-viz.service

    rm tmp.service
else
    echo "SCION-viz systemd service file not specified! SCION-viz won't run automatically on startup."
fi

if [[ $keep_user_context = true ]]
then
  sudo sh -c 'echo \"RemoveIPC=no\" >> /etc/systemd/logind.conf'
fi
