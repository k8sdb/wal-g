#!/bin/sh
set -xe

# install compiler
apk add --update --no-cache ca-certificates cmake bash build-base git

mkdir .brotli.tmp
cp -rf vendor/github.com/google/brotli/* .brotli.tmp
cp -rf .brotli/* vendor/github.com/google/brotli/

readonly CWD=$PWD
cd vendor/github.com/google/brotli/go/cbrotli
readonly LIB_DIR=../../dist  # dist will contain binaries and it is in the google/brotli/.gitignore
# patch cgo.go to force usage of static libraries for linking
sed -e "s|#cgo LDFLAGS: -lbrotlicommon|#cgo CFLAGS: -I../../c/include|" \
    -e "s|\(#cgo LDFLAGS:\) \(-lbrotli.*\)|\1 -L$LIB_DIR \2-static -lbrotlicommon-static|" \
    -e "/ -lm$/ n; /brotlienc/ s|$| -lm|" -i cgo.go

mkdir -p $LIB_DIR
cd $LIB_DIR
../configure-cmake --disable-debug
make
cd $CWD

make
