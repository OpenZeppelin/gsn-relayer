#!/bin/bash

# Exit script as soon as a command fails.
set -o errexit

relayer=$1.gsn.openzeppelin.org

pause() {
  read -p "Press Enter when done"
}

echo "Will upgrade relayer at $relayer"

echo "Stopping relayer service"
ssh ubuntu@$relayer 'sudo systemctl stop relayer'

echo "Uploading new binary"

rsync -avz -e "ssh -o StrictHostKeyChecking=no" --progress build/server/bin/RelayHttpServer  ubuntu@$relayer:/app/bin/RelayHttpServer

echo "Upload complete"

echo "Restarting relayer service"
ssh ubuntu@$relayer 'sudo systemctl restart relayer'
