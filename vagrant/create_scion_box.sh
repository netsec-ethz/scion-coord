#!/bin/bash

set -e

VERSION="0.0.1.2"
BASE=$(realpath $(dirname "$0"))
cd "$BASE"

do_create=1
do_package=0
usage="$(basename "$0") [-p]

where:
    -h          this help
    -p          also package the VM
    -j          only package the VM (don't create it)"

while getopts ":hpj" opt; do
    case $opt in
        p)
            do_package=1
            ;;
        j)
            do_create=0
            do_package=1
            ;;
        h)
            echo "$usage" >&2
            exit 0
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

# running this script will create a new vagrant virtual machine with scion installed and ready to work
# if do_package==1, it will also run `vagrant package` to get a .box file that can be uploaded to vagrant cloud
# for more info see https://www.vagrantup.com/docs/vagrant-cloud/boxes/create-version.html
if [ $do_create -eq 1 ]; then
    FILES=("../scion_install_script.sh"
           "../files/vagrant_box/scion.service"
           "../files/vagrant_box/scionupgrade.service"
           "../files/vagrant_box/scionupgrade.timer"
           "../files/vagrant_box/scionupgrade.sh"
           "../files/vagrant_box/scion-viz.service")
    for f in "${FILES[@]}"; do
        cp "$f" .
    done

    echo '------------------------------------ destroying old VMs'
    vagrant destroy -f
    VAGRANT_VAGRANTFILE=Vagrantfile-bootstrap vagrant destroy -f
    echo '------------------------------------ updating vagrant boxes'
    VAGRANT_VAGRANTFILE=Vagrantfile-bootstrap vagrant box update
    echo '------------------------------------ creating bootstrap vagrant VM'
    VAGRANT_VAGRANTFILE=Vagrantfile-bootstrap vagrant up
    echo '------------------------------------ creating base vagrant VM'
    vagrant up --provision

    for f in "${FILES[@]}"; do
        rm $(basename $f)
    done
fi

if [ $do_package -eq 1 ]; then
    echo '------------------------------------ packaging base vagrant VM'
    rm -f scion-base.box
    vagrant package --output scion-base.box --vagrantfile Vagrantfile-exported
    vagrant destroy -f
    echo '------------------------------------ locally adding base vagrant VM'
    vagrant box remove -f scion/ubuntu-16.04-64-scion --all || true
    cp metadata-template.json /tmp/metadata.json
    sed -i -- "s|_VERSION_|$VERSION|g" /tmp/metadata.json
    sed -i -- "s|_DIRFULLPATH_|$BASE|g" /tmp/metadata.json
    vagrant box add /tmp/metadata.json
fi
echo "Done."
