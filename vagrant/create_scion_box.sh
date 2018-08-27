#!/bin/bash

set -e

VERSION="0.0.1.2"

BASE=$(realpath $(dirname "$0"))
cd "$BASE"


# running this script will create a new vagrant virtual machine with scion installed and ready to work
# after this, you can run `vagrant package` to get a .box file that can be uploaded to vagrant cloud
# for more info see https://www.vagrantup.com/docs/vagrant-cloud/boxes/create-version.html
cp ../scion_install_script.sh .
vagrant destroy -f
echo '------------------------------------ updating vagrant boxes'
vagrant box update
echo '------------------------------------ creating vagrant VM OS'
VAGRANT_VAGRANTFILE=Vagrantfile-base vagrant up
VAGRANT_VAGRANTFILE=Vagrantfile-base vagrant halt
echo '------------------------------------ creating vagrant VM base'
vagrant up
echo '------------------------------------ packaging vagrant VM'
rm -f scion-base.box
# vagrant package --base SCIONLabVM-base --output scion-base.box --vagrantfile Vagrantfile-exported
vagrant package --output scion-base.box --vagrantfile Vagrantfile-exported
vagrant destroy -f


echo '------------------------------------ locally adding vagrant VM'
vagrant box remove scion/ubuntu-16.04-64-scion --box-version "$VERSION" || true
# vagrant box add "scion/ubuntu-16.04-64-scion" scion-base.box

cp metadata-template.json metadata.json
sed -i -- "s|_VERSION_|$VERSION|g" metadata.json
sed -i -- "s|_DIRFULLPATH_|$BASE|g" metadata.json
vagrant box add metadata.json
echo '------------------------------------'
echo "Done."


# curl 'https://vagrantcloud.com/api/v1/box/scion/ubuntu-16.04-64-scion/version/0.0.1.2/provider/virtualbox/upload?access_token=zaGkqfsnqqXIpg.atlasv1.vRq2XBHhOcx5YU8fABDZy88mQxVcXEFxbuyzrGGRCRJJRLf3XUKbgeGxY7fV2fzkd2w'

# curl -X PUT --upload-file scion-base.box 'https://archivist.vagrantup.com/v1/object/eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJrZXkiOiJib3hlcy80OTA3NzQwMi1mNTZkLTQ4Y2EtODVhMS03NTAyZDRiNGViZDgiLCJtb2RlIjoidyIsImV4cGlyZSI6MTUzNTMwNDY3NiwiY2FsbGJhY2siOiJodHRwczovL3ZhZ3JhbnRjbG91ZC5jb20vYXBpL2ludGVybmFsL2FyY2hpdmlzdC9jYWxsYmFjayJ9.JyhdnkITP8HcwF1wlp48cs9EX5MPIMewqX3ibCxIz-Q'

# curl 'https://vagrantcloud.com/api/v1/box/scion/ubuntu-16.04-64-scion/version/0.0.1.2/provider/virtualbox?access_token=zaGkqfsnqqXIpg.atlasv1.vRq2XBHhOcx5YU8fABDZy88mQxVcXEFxbuyzrGGRCRJJRLf3XUKbgeGxY7fV2fzkd2w'
