#!/bin/bash

# Exit script as soon as a command fails.
set -o errexit

relayer=$1.gsn.openzeppelin.org

pause() {
  read -p "Press Enter when done"
}

echo "Will upgrade relayer at $relayer"

echo "Connect via ssh to the relayer ('ssh ubuntu@$relayer') and run 'sudo systemctl stop relayer'"
pause

echo "Uploading new binary"

rsync -avz -e "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null" --progress build/server/bin/RelayHttpServer  ubuntu@$relayer:/app/bin/RelayHttpServer

echo "Update complete. Run 'sudo systemctl restart relayer' on the relayer"
pause

echo "Run 'journalctl -u relayer -r' on the relayer to view logs and check status"
