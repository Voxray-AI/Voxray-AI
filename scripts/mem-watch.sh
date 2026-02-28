#!/bin/bash

# Watch process RSS (resident set size) in GB. Usage: ./scripts/mem-watch.sh <PID>

PID=$1

if [ -z "$PID" ]; then
  echo "Usage: $0 <PID>"
  exit 1
fi

while true; do
  clear
  ps -p "$PID" -o pid,comm,rss 2>/dev/null | \
  awk 'NR==1 {print $0, "rss_GB"} NR>1 {printf "%s %s %s %.2f\n", $1,$2,$3,$3/1024/1024}'
  sleep 1
done
