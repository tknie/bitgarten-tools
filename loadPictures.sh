#!/bin/sh

POSTGRES_PORT=5434
POSTGRES_DB=photodb
POSTGRES_PASS=test123 
export POSTGRES_PORT POSTGRES_DB POSTGRES_PASS

bin/darwin_arm64/picloadql -A $* 2>&1|tee piclql.out

