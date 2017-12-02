#!/bin/bash

shopt -s nullglob

usage="$(basename "$0") [-p PATCH_DIR] [-g GEN_DIR] [-v VPN_CONF_PATH] [-s SCION_SERVICE] [-z SCION_VI_SERVICE]

where:
    -p PATCH_DIR        apply patches from PATCH_DIR on cloned repo
    -g GEN_DIR          path to gen directory to be used
    -v VPN_CONF_PATH    path to OpenVPN configuration file
    -s SCION_SERVICE    path to SCION service file
    -z SCION_VI_SERVICE path to SCION viz service file"

while getopts ":p:g:v:s:z:h" opt; do
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

sudo apt-get -y update
sudo apt-get -y install git

echo 'export GOPATH="$HOME/go"' >> ~/.profile
echo 'export PATH="$HOME/.local/bin:$GOPATH/bin:/usr/local/go/bin:$PATH"' >> ~/.profile
echo 'export SC="$GOPATH/src/github.com/netsec-ethz/scion"' >> ~/.profile
echo 'export PYTHONPATH="$SC/python:$SC"' >> ~/.profile
source ~/.profile
mkdir -p "$GOPATH"
mkdir -p "$GOPATH/src/github.com/netsec-ethz"
cd "$GOPATH/src/github.com/netsec-ethz"

git config --global url.https://github.com/.insteadOf git@github.com:
git clone --recursive -b scionlab git@github.com:netsec-ethz/scion

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

    git_username=$(git config user.names)

    # We need to have git user in order to commit
    if [ -z "$git_username" ]
    then
        echo "GIT user credentials not set, configuring defaults"
        git config --global user.name "Scion User" 
        git config --global user.email "scion@scion-architecture.net"
    fi

    git commit -am "Modified to compile on ARM systems"

    echo "Finished applying patches"
fi

bash -c 'yes | GO_INSTALL=true ./env/deps'

sudo cp docker/zoo.cfg /etc/zookeeper/conf/zoo.cfg

# Check if gen directory exists
if  [[ ( ! -z ${gen_dir+x} ) && -d ${gen_dir} ]]
then  
    echo "Gen directory is specified! Using content from there!"
    
    cp -r "$gen_dir" .
else
    echo "Gen directory is NOT specified! Generating local topology!"
    
    ./scion.sh topology
fi

cd sub
git clone git@github.com:netsec-ethz/scion-viz
cd scion-viz/python/web
pip3 install --user --require-hashes -r requirements.txt
python3 ./manage.py migrate

echo "alias cdscion='cd $SC'" >> ~/.bash_aliases
echo "alias checkbeacons='tail -f $SC/logs/bs*.DEBUG'" >> ~/.bash_aliases

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
    echo "Registring SCION as startup service"

    cp "$scion_service_path" tmp.service
    # We need to replace ubuntu user with current username
    sed -i "s/ubuntu/$USER/g" tmp.service
    sudo cp tmp.service /etc/systemd/system/scion.service

    sudo systemctl enable scion.service
    sudo systemctl start scion.service

    rm tmp.service
else
    echo "SCION systemd service file not specified! SCION won't run automatically on startup."
fi

if  [[ ( ! -z ${scion_viz_service+x} ) && -r ${scion_viz_service} ]]
then
    echo "Registring SCION viz as startup service"

    cp "$scion_viz_service" tmp.service
    # We need to replace ubuntu user with current username
    sed -i "s/ubuntu/$USER/g" tmp.service
    sudo cp tmp.service /etc/systemd/system/scion-viz.service

    sudo systemctl enable scion-viz.service
    sudo systemctl start scion-viz.service
else
    echo "SCION viz systemd service file not specified! SCION won't run automatically on startup."
fi