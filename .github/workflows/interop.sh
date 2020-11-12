#!/bin/bash

set -e 

SERVER=$1
CLIENT=$2
TEST=$3
SERVER_ADDR="/ip4/127.0.0.1/udp/12345/quic"

for k1 in server*.key; do
  for k2 in client*.key; do
    if [[ $TEST == "handshake-failure" ]]; then
      for k3 in other*.key; do
        echo -e "\nRunning with server $SERVER ($k1) and client $CLIENT ($k2). Test: handshake-failure"
        ./$SERVER -role server -key $k1 -peerkey $k2 -addr $SERVER_ADDR -test handshake-failure &
        PID=$!
        time ./$CLIENT -role client -key $k2 -peerkey $k3 -addr $SERVER_ADDR -test handshake-failure
        kill $PID
        wait
      done;
    else
      echo -e "\nRunning with server $SERVER ($k1) and client $CLIENT ($k2). Test: $TEST"
      ./$SERVER -role server -key $k1 -peerkey $k2 -addr $SERVER_ADDR -test $TEST &
      PID=$!
      time ./$CLIENT -role client -key $k2 -peerkey $k1 -addr $SERVER_ADDR -test $TEST
      wait $PID
    fi
  done;
done;
