#!/bin/bash

# Exit script as soon as a command fails.
set -o errexit

for network in 'xdai-01' 'kovan-01' 'rinkeby-02' 'mainnet-02' 'mainnet-03'; do
  echo "Upgrading $network"
  ./scripts/upgrade-relayer.sh $network
  echo "Upgrade complete"
  echo
done
