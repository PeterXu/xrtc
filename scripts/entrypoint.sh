#!/bin/bash

export GOPATH="/gopath"

[ -e "/gobuild" ] && cd "/gobuild"

exec "$@"
