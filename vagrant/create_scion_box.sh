#!/bin/bash

# running this script will create a new vagrant virtual machine with scion installed and ready to work
# after this, you can run `vagrant package` to get a .box file that can be uploaded to vagrant cloud
# for more info see https://www.vagrantup.com/docs/vagrant-cloud/boxes/create-version.html
cp ../scion_install_script.sh .
vagrant destroy -f
echo '------------------------------------'
vagrant box update
echo '------------------------------------'
vagrant up
