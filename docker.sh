#!/bin/bash

VERSION=0.1

BASE="$(dirname $(realpath $0))"

build_scion_image() {
    pushd $SC >/dev/null
    ./docker.sh base
    ./docker.sh build
    popd >/dev/null
}

build() {
    build_scion_image

    # # docker build -t juagargi/scionlab-scion:0.1 -f docker/Dockerfile-scion .
    docker build -t scionlab-coord:0.1 -f docker/Dockerfile-coord . || exit 1
    docker build -t scionlab-coord-test -f docker/Dockerfile-coord-test . || exit 1
    # docker push juagargi/scionlab-coord:0.1
}

rebuild() {
    # for now, just remove the image and build
    docker rmi scionlab-coord:0.1 scionlab-coord-test || true
    build
}

test() {
    # asfdasd
    cd $BASE
    docker-compose -f docker/test-coordinator.yml up --abort-on-container-exit --exit-code-from test
    TEST1=$?
    echo "Test1 exit status: $TEST1"
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


