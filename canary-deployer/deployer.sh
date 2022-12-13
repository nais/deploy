#!/bin/sh

export VARS=`mktemp`

for CLUSTER in $CLUSTERS; do
  export CLUSTER
  echo "---" > $VARS
  echo "now: $(date +%s)000000000" >> $VARS
  echo "namespace: ${NAMESPACE}" >> $VARS
  echo "Deploying to $NAMESPACE in $CLUSTER..."
  /app/deploy --wait=false
done
