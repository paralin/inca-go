#!/bin/bash
set -eo pipefail

rm -rf node2/
mkdir -p ./node2/
cd ./node2
cp ../chain.json ./
../inca node --init-node-config
