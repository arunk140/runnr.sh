#!/bin/bash
IN="$@"
IFS='&&' read -ra ADDR <<< "$IN"
for i in "${ADDR[@]}"; do
    if [ -z "$i" ]; then
        continue
    fi
    echo "Running: $i"
    eval "$i"
    echo "exit code: $?"
done

pwd