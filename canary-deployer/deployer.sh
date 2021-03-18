#!/bin/sh

export VARS=`mktemp`

for CLUSTER in $CLUSTERS; do
  export CLUSTER
  echo "---" > $VARS
  echo "now: $(date +%s)000000000" >> $VARS
  echo "Deploying to $CLUSTER..."
  /app/deploy --wait=false
done
