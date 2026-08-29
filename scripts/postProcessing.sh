#!/bin/sh

. $(dirname $0)/../.env

POSTGRES_PORT=5432
POSTGRES_DB=photodb
POSTGRES_PASS=test123 
#POSTGRES_URL=postgres://admin:test123@photodb.fritz.box:5434/photodb
POSTGRES_URL=postgres://admin:test123@10.10.50.201:5432/photodb

export POSTGRES_PORT POSTGRES_DB POSTGRES_PASS POSTGRES_URL
#BITGARTEN_SERVER=http://services.fritz.box:8030/fotos
BITGARTEN_SERVER=http://services.fritz.box:8030
BITGARTEN_USERNAME=admin
BITGARTEN_PASSWORD=test123
BITGARTEN_LOCATION=photodb
export BITGARTEN_SERVER BITGARTEN_USERNAME BITGARTEN_PASSWORD BITGARTEN_LOCATION
LOGPATH=`pwd`/logs
#ENABLE_DEBUG=1
#export ENABLE_DEBUG LOGPATH
#ENABLE_LOGLEVEL=4
#export LOGPATH  ENABLE_LOGLEVEL
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
_GO_X=$GOOS"_"$GOARCH
echo $_GO_X

echo "sh: Starting hash clean pictures  ..."
bin/$_GO_X/hashclean -l 0 -C
if [ "$?" != "0" ]; then
   exit 1
fi

echo "sh: Starting hash clean HEIC  ..."
bin/$_GO_X/hashclean -l 0 -C -H 
if [ "$?" != "0" ]; then
   exit 1
fi

echo "sh: Starting image hash ..."
bin/$_GO_X/imagehash -C -l 0
if [ "$?" != "0" ]; then
   exit 1
fi

bin/$_GO_X/videothumb -C
if [ "$?" != "0" ]; then
   exit 1
fi

