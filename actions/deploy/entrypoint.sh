#!/bin/sh
# vi: se et:

echo "::add-mask::$APIKEY"

if [ -z "$OWNER" ]; then
    export OWNER=`echo $GITHUB_REPOSITORY | cut -f1 -d/`
fi

if [ -z "$REPOSITORY" ]; then
    export REPOSITORY=`echo $GITHUB_REPOSITORY | cut -f2 -d/`
fi

if [ -z "$QUIET" ]; then
    export QUIET=false
fi

if [ -z "$DRY_RUN" ]; then
    export DRY_RUN=false
fi

if [ -z "$PRINT_PAYLOAD" ]; then
    export PRINT_PAYLOAD=false
fi

# Inject "image" as a template variable to a new copy of the vars file.
# If the file doesn't exist, it is created. The original file is left untouched.
if [ ! -z "$IMAGE" ]; then
    export VARS_ORIGINAL="$VARS"
    export VARS=`mktemp`
    if [ -z "$VARS_ORIGINAL" ]; then
        echo "---" > $VARS
    else
        cat $VARS_ORIGINAL > $VARS
    fi
    yq w --inplace $VARS image "$IMAGE"
fi

/app/deploy \
    --actions="true" \
    --apikey="$APIKEY" \
    --cluster="$CLUSTER" \
    --dry-run="$DRY_RUN" \
    --owner="$OWNER" \
    --print-payload="$PRINT_PAYLOAD" \
    --quiet="$QUIET" \
    --ref="$GITHUB_REF" \
    --repository="$REPOSITORY" \
    --resource="$RESOURCE" \
    --team="$TEAM" \
    --vars="$VARS" \
    --wait="true" \
    ;
