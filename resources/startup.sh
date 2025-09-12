#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# Prints the cloudogu logo as ASCI art.
function printCloudoguLogo() {
  echo "                                     ./////,                    "
  echo "                                 ./////==//////*                "
  echo "                                ////.  ___   ////.              "
  echo "                         ,**,. ////  ,////A,  */// ,**,.        "
  echo "                    ,/////////////*  */////*  *////////////A    "
  echo "                   ////'        \VA.   '|'   .///'       '///*  "
  echo "                  *///  .*///*,         |         .*//*,   ///* "
  echo "                  (///  (//////)**--_./////_----*//////)   ///) "
  echo "                   V///   '°°°°      (/////)      °°°°'   ////  "
  echo "                    V/////(////////\. '°°°' ./////////(///(/'   "
  echo "                       'V/(/////////////////////////////V'      "
}

# Provides an easy function to create consistent log messages.
function log() {
  local message="${1}"
  echo "[nginx-static][startup] ${message}"
}

# Configures the warp menu script as the menu.json gets mounted from a configmap into "/var/www/html/warp/menu"
# instead of "/var/www/html/warp". This is a special constraints when mounting config maps. Mounting the warp menu
# json directly into the warp folder would directly delete other files in the warp folder, including the warp.js script.
function configureWarpMenu() {
  log "Configure warp menu..."

  # Replace /warp/menu.json with /warp/menu/menu.json
  sed -i "s|/warp/menu.json|/warp/menu/menu.json|g" /var/www/html/warp/warp.js
}

# Starts the nginx server.
function startNginx() {
  log "Starting nginx service..."
  exec nginx -c /etc/nginx/nginx.conf -g "daemon off;"
}

# make the script only run when executed, not when sourced from bats tests.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  printCloudoguLogo
  configureWarpMenu
  startNginx
fi
