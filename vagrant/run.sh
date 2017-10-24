#!/usr/bin/env bash

# BEGIN subcommand functions

VB="virtualbox"
VG="vagrant"
VM="vagrant-manager"

run_vagrant() {
    echo "[SCIONLabVM] run vagrant"
    vagrant box update
    vagrant up
    vagrant ssh
}

run_osx() {
    echo "[SCIONLabVM] Given system: OSX"
    if ! type "brew" > /dev/null; then
        echo "[SCIONLabVM] Now installing Homebrew"
        ruby -e "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)"
    fi
    for pkg in $VB $VG $VM; do
        if pkgutil --pkgs $pkg > /dev/null; then
            echo "[SCIONLabVM] $pkg is already installed"
        elif brew cask ls $pkg > /dev/null; then
            echo "[SCIONLabVM] $pkg is already installed"
        else
            echo "[SCIONLabVM] Installing $pkg"
            brew cask install --force $pkg
        fi
    done
    run_vagrant
}

run_linux() {
    if [[ -f "/usr/bin/apt-get" && -f "/usr/bin/dpkg" ]]
    then
        echo "[SCIONLabVM] Given system: LINUX"
        sudo bash -c 'echo deb http://vagrant-deb.linestarve.com/ any main > /etc/apt/sources.list.d/wolfgang42-vagrant.list'
        sudo apt-key adv --keyserver pgp.mit.edu --recv-key AD319E0F7CFFA38B4D9F6E55CE3F3DE92099F7A4
        sudo apt-get update
        if dpkg --get-selections | grep -q "^$VB[[:space:]]*install$" >/dev/null; then
            echo "[SCIONLabVM] $VB is already installed"
        else
            echo "[SCIONLabVM] Installing $VB"
            sudo apt-get --yes install $VB
        fi
        if vagrant version | grep "Installed Version: 1.9" >/dev/null; then
            echo "[SCIONLabVM] $VG is already installed"
        elif dpkg --get-selections | grep -q "^$VG[[:space:]]*install$" >/dev/null; then
            echo "[SCIONLabVM] ${RED}Warning!${NC} Current version of $VG in your system is out of date."
            while true; do
                read -p "[SCIONLabVM] Do you want upgrade $VG now? If no, it will terminate SCIONLabVM immediately. [y/n]" yesno
                case $yesno in
                    [Yy]*) sudo apt-get install --only-upgrade $VG; break;;
                    [Nn]*) echo "[SCIONLabVM] Closing SCIONLabVM installation."; exit 1;;
                    *) ;;
                esac
            done
        else
            echo "[SCIONLabVM] Installing $VG"
            sudo apt-get --yes install $VG
        fi
        run_vagrant
    else
        echo "Currently, SCIONLabVM does not support your linux distribution."
    fi
}


case "$OSTYPE" in
  darwin*)
        "run_osx" ;;
  linux*)
        "run_linux" ;;
  solaris*|bsd*|msys|*) 
    echo "Currently, SCIONLabVM does not support $OSTYPE system." ;;
esac
