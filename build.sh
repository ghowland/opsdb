#!/bin/bash

make build
make test
make fmt
make lint
make build-one CMD=./tools/opsdb-api/cmd OUT=opsdb-api
make run CMD=./tools/opsdb-schema/cmd

