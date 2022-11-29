#!/bin/bash

echo "::group::Build katana"
rm integration-test katana 2>/dev/null
cd ../cmd/katana
go build
mv katana ../../integration_tests/katana
echo "::endgroup::"

echo "::group::Build katana integration-test"
cd ../integration-test
go build
mv integration-test ../../integration_tests/integration-test
cd ../../integration_tests
echo "::endgroup::"

./integration-test
if [ $? -eq 0 ]
then
  exit 0
else
  exit 1
fi
