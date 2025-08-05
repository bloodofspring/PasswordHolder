#! /usr/bin/bash

touch start_log.txt
git pull

if go version 2> start_log.txt
then
  echo "Go detected. Starting up the project..."
  go run run.go
else
  echo "No go interpreter detected! Install go before use"
fi
