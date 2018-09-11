#!/bin/bash

run_ok() {
  echo "Waiting..."
  sleep 2
  echo "Done"
}

run_err() {
  echo "Waiting..."
  sleep 2
  echo "Raised error" 1>&2
  exit 2
}

run_help() {
  echo "Unknown command: $cmd" 1>&2
  sleep 1
  echo -e "Allowed commands:\n* test\n* error\n* uptime"
  exit 1
}

cmd=$1
case "$cmd" in
  test)
    run_ok
    ;;
  error)
    run_err
    ;;
  *)
    run_help
    ;;
esac
