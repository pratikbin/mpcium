#!/bin/bash

# Directories to clean under
nodes=("node0" "node1" "node2")

for dir in "${nodes[@]}"; do
  identity_dir="$dir"
  echo "Cleaning .txt files in $identity_dir..."
  if [ -d "$identity_dir" ]; then
    find "$identity_dir" -type f -name "*.txt" -print -delete
  else
    echo "Directory $identity_dir not found"
  fi
done

echo "✅ Cleanup complete."
