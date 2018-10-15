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

UPGRADE_SCRIPT_LOCATION="/usr/bin/scionupgrade.sh"

usage="$(basename "$0") [-p PATCH_DIR] [-g GEN_DIR] [-v VPN_CONF_PATH] \
[-s SCION_SERVICE] [-z SCION_VI_SERVICE] [-a ALIASES_FILE] [-c] \
[-u UPGRADE_SCRIPT] [-t TIMER_SERVICE]

where:
    -p PATCH_DIR        apply patches from PATCH_DIR on cloned repo
    -g GEN_DIR          path to gen directory to be used
    -v VPN_CONF_PATH    path to OpenVPN configuration file
    -s SCION_SERVICE    path to SCION service file
    -z SCION_VI_SERVICE path to SCION-viz service file
    -a ALIASES_FILE     adds useful command aliases in specified file
    -c                  do not destroy user context on logout
    -u UPGR_SCRIPT      script used for upgrading scion, (will be copied to 
                        path ${UPGRADE_SCRIPT_LOCATION})
    -t TIMER_UPG_SERV   name of sysd timer and system name for upgrades"

while getopts ":p:g:v:s:z:ha:cu:t:" opt; do
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
    u)
      upgrade_script=$OPTARG
      ;;
    t)
      upgrade_timer=${OPTARG}.timer
      upgrade_service=${OPTARG}.service
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
if ! dpkg --version &>/dev/null || ! apt-get --version &>/dev/null; then
    echo "Unsupported system."
    exit 1
fi

command -v git >/dev/null 2>&1 || sudo apt-get -y install git

[ -f ~/.profile ] || touch ~/.profile
source ~/.profile
echo "$GOPATH" | grep "$HOME/go" &> /dev/null || echo 'export GOPATH="$HOME/go"' >> ~/.profile
echo "$PATH" | grep "/usr/local/go/bin" &> /dev/null || echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.profile
echo "$PATH" | grep "$GOPATH/bin" &> /dev/null || echo 'export PATH="$GOPATH/bin:$PATH"' >> ~/.profile
echo "$PATH" | grep "$HOME/.local/bin" &> /dev/null || echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.profile
echo "$SC" | grep "$GOPATH/src/github.com/scionproto/scion" &> /dev/null || echo 'export SC="$GOPATH/src/github.com/scionproto/scion"' >> ~/.profile
echo $PYTHONPATH | grep "$SC/python" &> /dev/null || echo 'export PYTHONPATH="$SC/python:$SC"' >> ~/.profile

source ~/.profile
# if you don't have a user configured, set up http to clone anonymously:
git config -l | grep 'user.email' &>/dev/null || git config --global url.https://github.com/.insteadOf git@github.com:

mkdir -p "$GOPATH/src/github.com/scionproto"
cd "$GOPATH/src/github.com/scionproto"
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

tempfile=$(mktemp)
if  [[ ( ! -z ${scion_service_path+x} ) && -r ${scion_service_path} ]]
then
    echo "Registering SCION as startup service"

    cp "$scion_service_path" "$tempfile"
    # We need to replace template user with current username
    sed -i "s/_USER_/$USER/g" "$tempfile"
    sudo cp "$tempfile" /etc/systemd/system/scion.service

    sudo systemctl enable scion.service
    sudo systemctl start scion.service

    rm "$tempfile"
else
    echo "SCION systemd service file not specified! SCION won't run automatically on startup."
fi

if  [[ ( ! -z ${scion_viz_service+x} ) && -r ${scion_viz_service} ]]
then
    echo "Registering SCION-viz as startup service"

    cp "$scion_viz_service" "$tempfile"
    # We need to replace template user with current username
    sed -i "s/_USER_/$USER/g" "$tempfile"
    sudo cp "$tempfile" /etc/systemd/system/scion-viz.service

    sudo systemctl enable scion-viz.service
    sudo systemctl start scion-viz.service

    rm "$tempfile"
else
    echo "SCION-viz systemd service file not specified! SCION-viz won't run automatically on startup."
fi

if [[ $keep_user_context = true ]]
then
  sudo sh -c 'echo RemoveIPC=no >> /etc/systemd/logind.conf'
fi

if  [[ ( ! -z ${upgrade_script+x} ) ]]
then
    echo "Copying scion upgrade script"

    chmod +x ${upgrade_script}
    sudo cp ${upgrade_script} ${UPGRADE_SCRIPT_LOCATION}
else
    echo "SCION upgrade script not specified."
fi

if  [[ ( ! -z ${upgrade_service+x} ) && -r ${upgrade_service} \
    && ( ! -z ${upgrade_timer+x} ) && -r ${upgrade_timer} \
    && ( ! -z ${UPGRADE_SCRIPT_LOCATION+x} ) && -r ${UPGRADE_SCRIPT_LOCATION} ]]
then
    echo "Registering SCION periodic upgrade service"

    cp "$upgrade_service" "$tempfile"
    sed -i "s/_USER_/$USER/g" "$tempfile"
    sudo cp "$tempfile" /etc/systemd/system/scionupgrade.service
    rm "$tempfile"

    cp "$upgrade_timer" "$tempfile"
    sed -i "s/_USER_/$USER/g" "$tempfile"
    sudo cp "$tempfile" /etc/systemd/system/scionupgrade.timer
    rm "$tempfile"

    sudo systemctl enable scionupgrade.timer
    sudo systemctl enable scionupgrade.service
    
    sudo systemctl start scionupgrade.timer
    sudo systemctl start scionupgrade.service

    if [ -d "/vagrant" ]; then # iff this is a VM
        # registering the upgrade service also means "manage SCION", including keep time sync'ed
        sudo apt-get install -y --no-remove ntp || true
        sudo sed -i -- 's/^\(\s*start-stop-daemon\s*--start\s*--quiet\s*--oknodo\s*--exec\s*\/usr\/sbin\/VBoxService\)$/\1 -- --disable-timesync/g' /etc/init.d/virtualbox-guest-utils || true
        # restart virtual box guest services and NTPd :
        sudo systemctl daemon-reload || true
        sudo systemctl restart virtualbox-guest-utils
        sudo systemctl enable ntp || true
        # we want ntpd to use the -g flag (no panic threshold):
        if ! egrep -- '^NTPD_OPTS=.*-g.*$' /etc/default/ntp >/dev/null; then
            sudo sed -i "s/^NTPD_OPTS='\(.*\)'/NTPD_OPTS=\'\\1\ -g'/g" /etc/default/ntp
        fi
        if ! grep 'tinker panic 0' /etc/ntp.conf; then
            # set panic limit to 0 (disable)
            echo -e "tinker panic 0\n" | sudo tee -a /etc/ntp.conf >/dev/null
        fi
        if ! egrep -- '^pool.*maxpoll.*$' /etc/ntp.conf; then
            sudo sed -i 's/\(pool .*\)$/\1 minpoll 1 maxpoll 6/g' /etc/ntp.conf
        fi
        sudo systemctl restart ntp || true
        # system updates, ensure unattended-upgrades is installed
        if ! dpkg-query -W --showformat='${Status}\n' unattended-upgrades|grep "install ok installed" >/dev/null; then
            sudo apt-get install -f --no-remove unattended-upgrades
        fi
        if [ ! -f /etc/apt/apt.conf.d/51unattended-upgrades ]; then
            echo "Configuring unattended-upgrades"
            echo 'Unattended-Upgrade::Allowed-Origins {
"${distro_id}:${distro_codename}-security";
"${distro_id}ESM:${distro_codename}";
};
Unattended-Upgrade::Automatic-Reboot "true";
Unattended-Upgrade::Automatic-Reboot-Time "02:00";' | sudo tee /etc/apt/apt.conf.d/51unattended-upgrades >/dev/null
        fi
    fi
else
    echo "SCION periodic upgrade service and timer files are not provided."
fi

# check if zookeeper is running:
if ! /usr/share/zookeeper/bin/zkServer.sh status &>/dev/null; then
    echo "Zookeeper not started. Starting the service."
    sudo systemctl restart zookeeper || true # restart, because we want to ensure it's running
fi
