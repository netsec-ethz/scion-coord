#!/bin/bash

VERSION=0.2

BASE="$(dirname $(realpath $0))"
build_dir="./docker/_build"
dst_path="${build_dir}/scion-coord.git/"


# version less or equal. E.g. verleq 1.9 2.0.8  == true (1.9 <= 2.0.8)
verleq() {
    [ ! -z "$1" ] && [ ! -z "$2" ] && [ "$1" = `echo -e "$1\n$2" | sort -V | head -n1` ]
}

check_docker_binaries() {
    # example of `docker --version` :
    # Docker version 17.03.2-ce, build f5ec1e2
    V=$(docker --version | awk '{print $3}')
    verleq "18.01" "$V" && return 0 || return 1
}

install_docker_binaries() {
    sudo apt-get -y purge 'docker*'
    echo 'deb [arch=amd64] https://download.docker.com/linux/ubuntu xenial stable' | sudo tee /etc/apt/sources.list.d/docker.list
    sudo apt-get update
    sudo apt-get install -y docker-ce
    # sudo apt-get -y install docker.io
    sudo usermod -aG docker "$USER"
}

build() {
    build_scion_image

    echo "Building Coordinator Docker images"
    copy_tree
    build_specific '-base'
    build_specific
    build_specific '-test'
}

build_specific() {
    local specific=$1
    local suffix=${specific:-$specific}
    local tag=scionlab-coord$suffix:$VERSION
    echo
    echo "Building scionlab-coord$suffix:$VERSION Docker image"
    echo "=========================="
    docker build -t $tag -f docker/Dockerfile-coord$suffix . || exit 1
    docker tag "$tag" "scionlab-coord$suffix:latest"
}

rebuild() {
    # for now, just remove the image and build
    docker rmi scionlab-coord:$VERSION scionlab-coord-test || true
    build
}

test() {
    cd $BASE
    docker-compose -f docker/test-coordinator.yml up --abort-on-container-exit --exit-code-from test
    TEST1=$?
    echo "Test1 exit status: $TEST1"
}


build_scion_image() {
    pushd $SC >/dev/null
    ./docker.sh base
    ./docker.sh build
    echo
    popd >/dev/null
}

copy_tree() {
    set -e
    set -o pipefail
    echo "Copying current working tree for Docker image"
    echo "============================================="
    [[ ! -f ./sub/scion-box/.git ]] && { echo "Run `git submodule update` and try again"; exit 1; }
    mkdir -p "${build_dir:?}"
    # Just in case it's sitting there from a previous run
    rm -rf "$dst_path"
    {
        git ls-files;
        git submodule --quiet foreach 'git ls-files | sed "s|^|$path/|"';
    } | rsync -a --files-from=- . "$dst_path"
    echo
}



usage="$(basename $0) {build|rebuild|test}

where:
    build           builds the containers
    rebuild         force-builds the Coordinator and test containers
    test            runs the tests"

f=''
case "$1" in
    build)          f=build ;;
    rebuild)        f=rebuild ;;
    test)           f=test ;;
    *)              echo "$usage"; exit 0 ;;
esac

check_docker_binaries || { echo "Installing updated docker ..."; install_docker_binaries; }
# run the function stored above
$f

