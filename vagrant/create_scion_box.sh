#!/bin/bash

set -e

# running this script will create a new vagrant virtual machine with scion installed and ready to work
# after this, you can run `vagrant package` to get a .box file that can be uploaded to vagrant cloud
# for more info see https://www.vagrantup.com/docs/vagrant-cloud/boxes/create-version.html
cp ../scion_install_script.sh .
vagrant destroy -f
echo '------------------------------------ updating vagrant boxes'
vagrant box remove scion/ubuntu-16.04-64-scion --box-version 0.0.1.2 || true
vagrant box update
echo '------------------------------------ creating vagrant VM OS'
VAGRANT_VAGRANTFILE=Vagrantfile-os vagrant up
VAGRANT_VAGRANTFILE=Vagrantfile-os vagrant halt
echo '------------------------------------ creating vagrant VM base'
VAGRANT_VAGRANTFILE=Vagrantfile vagrant up


exit 0


echo '------------------------------------ packaging vagrant VM'
rm -f scion-base.box
vagrant package --base SCIONLabVM-base --output scion-base.box --vagrantfile Vagrantfile-exported
vagrant destroy -f
exit 0
echo '------------------------------------ locally adding vagrant VM'
vagrant box remove scion/ubuntu-16.04-64-scion
# vagrant box add "scion/ubuntu-16.04-64-scion" scion-base.box
vagrant box add metadata.json
echo '------------------------------------'
echo "Done."
