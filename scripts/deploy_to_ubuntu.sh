#!/bin/sh
# deploy_to_ubuntu.sh — run ON the SDR server (or via ssh): rebuild + verify.
# Usage: ./scripts/deploy_to_ubuntu.sh  (on server, repo at ~/gsm-system)
set -eu
cd "$(dirname "$0")/.."
sudo docker compose -f deploy/docker/docker-compose.yml build
sudo docker compose -f deploy/docker/docker-compose.yml up -d
sleep 3
curl -s http://127.0.0.1:8082/api/v1/health; echo
