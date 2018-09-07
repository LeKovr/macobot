#!/bin/bash

echo "Called"

run_ok() {
sleep 1
echo Step1
sleep 2
echo Done
}

run_err() {
sleep 2
echo "Run rel error"
exit 1
}

run_ok
