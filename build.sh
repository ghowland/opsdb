#!/bin/bash

make build
make test
make fmt
make lint
make build-one CMD=./tools/opsdb_api/cmd OUT=opsdb_api
make run CMD=./tools/opsdb_schema/cmd

