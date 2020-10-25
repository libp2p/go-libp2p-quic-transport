#!/bin/bash

set -e 

SERVER=$1
CLIENT=$2
SERVER_ADDR="/ip4/127.0.0.1/udp/12345/quic"

for k1 in server*.key; do
  for k2 in client*.key; do
    # Run the handshake failure test.
    for k3 in other*.key; do
      echo "Running with server $SERVER ($k1) and client $CLIENT ($k2). Test: handshake-failure"
      ./$SERVER -role server -key $k1 -peerkey $k2 -addr $SERVER_ADDR -test handshake-failure &
      PID=$!
      ./$CLIENT -role client -key $k2 -peerkey $k3 -addr $SERVER_ADDR -test handshake-failure
      kill $PID
    done;
    # Run the transfer test.
    echo "Running with server $SERVER ($k1) and client $CLIENT ($k2)"
    ./$SERVER -role server -key $k1 -peerkey $k2 -addr $SERVER_ADDR -test single-transfer &
    ./$CLIENT -role client -key $k2 -peerkey $k1 -addr $SERVER_ADDR -test single-transfer
    wait &
  done;
done;
