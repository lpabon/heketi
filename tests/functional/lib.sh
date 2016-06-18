
fail() {
    echo "==> ERROR: $@"
    exit 1
}

println() {
    echo "==> $1"
}

HEKETI_PID=
start_heketi() {
    # Build server if we need to
    if [ ! -x "$HEKETI_SERVER" ] ; then
        ( cd $HEKETI_SERVER_BUILD_DIR && make server && cp heketi $HEKETI_SERVER )
        if [ $? -ne 0 ] ; then
            fail "Unable to build Heketi"
        fi
    fi

    # Start server
    rm -f heketi.db > /dev/null 2>&1
    $HEKETI_SERVER -config=config/heketi.json &
    HEKETI_PID=$!
    sleep 2
}

start_vagrant() {
    cd vagrant
    sudo -E ./up.sh || fail "unable to start vagrant virtual machines"
    cd ..
}

teardown_vagrant() {
    cd vagrant
    sudo vagrant destroy -f
    cd ..
}

run_tests() {
    cd tests
    go test -timeout=1h -tags functional
    gotest_result=$?
    cd ..
}

functional_tests() {
    teardown_vagrant
    start_vagrant
    start_heketi

    run_tests

    kill $HEKETI_PID
    teardown_vagrant

    exit $gotest_result
}

