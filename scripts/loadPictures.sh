#!/bin/sh


. $(dirname $0)/../.env

echo "sh: Start loading pictures  ..."
bin/darwin_arm64/picloadql -b 3G -A $* 2>&1|tee piclql.out
if [ "$?" != "0" ]; then
   exit 1
fi

#echo "sh: Starting hash clean pictures  ..."
#bin/darwin_arm64/hashclean -l 0 -C
#if [ "$?" != "0" ]; then
#   exit 1
#fi

#echo "sh: Starting hash clean HEIC  ..."
#bin/darwin_arm64/hashclean -l 0 -C -H 
#if [ "$?" != "0" ]; then
#   exit 1
#fi
#
#echo "sh: Starting image hash ..."
#bin/darwin_arm64/imagehash -C -l 0
#if [ "$?" != "0" ]; then
#   exit 1
#fi

