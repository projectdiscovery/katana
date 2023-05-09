#!/bin/bash

# reading os type from arguments
CURRENT_OS=$1

if [ "${CURRENT_OS}" == "windows-latest" ];then
    extension=.exe
fi

echo "::group::Building functional-test binary"
go build -o functional-test$extension
echo "::endgroup::"

echo "::group::Building katana binary from current branch"
go build -o katana_dev$extension ../katana
echo "::endgroup::"


echo 'Starting katana functional test'
./functional-test$extension -dev ./katana_dev$extension
