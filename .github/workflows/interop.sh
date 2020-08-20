#!/bin/bash

set -e 

SERVER=$1
CLIENT=$2
SERVER_ADDR="/ip4/127.0.0.1/udp/12345/quic"

for k1 in server*.key; do
  for k2 in client*.key; do
    echo "Running with server $SERVER ($k1) and client $CLIENT ($k2)"
    ./$SERVER -role server -key $k1 -peerkey $k2 -addr $SERVER_ADDR &
    ./$CLIENT -role client -key $k2 -peerkey $k1 -addr $SERVER_ADDR
    wait &
  done;
done;
