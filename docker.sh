#!/bin/bash

VERSION=0.2

BASE="$(dirname $(realpath $0))"
build_dir="./docker/_build"
dst_path="${build_dir}/scion-coord.git/"


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

case "$1" in
    build)          build ;;
    rebuild)        rebuild ;;
    test)           test ;;
    *)              echo "$usage";;
esac


