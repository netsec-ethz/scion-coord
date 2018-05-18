#!/bin/bash

VERSION=0.1

BASE="$(dirname $(realpath $0))"

build_scion_image() {
    pushd $SC >/dev/null
    # need_base=0
    # [[ $(docker images -q scion:scionlab) != "" ]] && need_scion=0 || { need_scion=1 && [[ $(docker images -q scion_base:scionlab) != "" ]] || need_base=1 ; } 
    # echo $need_base
    # echo $need_scion
    # if [ $need_base == 1 ]; then
    #     ./docker.sh base
    # fi
    # if [ $need_scion == 1 ]; then
    #     ./docker.sh build
    # fi
    ./docker.sh base
    ./docker.sh build
    popd >/dev/null
}

build() {
    build_scion_image

    # # docker build -t juagargi/scionlab-scion:0.1 -f docker/Dockerfile-scion .
    docker build -t scionlab-coord:0.1 -f docker/Dockerfile-coord .
    docker build -t scionlab-coord-test -f docker/Dockerfile-coord-test .
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


