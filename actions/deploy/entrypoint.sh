#!/bin/sh
# vi: se et:

echo "::add-mask::$APIKEY"

if [ -z "$OWNER" ]; then
    export OWNER=`echo $GITHUB_REPOSITORY | cut -f1 -d/`
fi

if [ -z "$REPOSITORY" ]; then
    export REPOSITORY=`echo $GITHUB_REPOSITORY | cut -f2 -d/`
fi

if [ -z "$REF" ]; then
    export REF="$GITHUB_REF"
fi

if [ -z "$WAIT" ]; then
    export WAIT="true"
fi

if [ -z "$GITHUB_SHA" ]; then
    export GIT_REF_SHA="$GITHUB_SHA"
fi

# Inject "image" as a template variable to a new copy of the vars file.
# If the file doesn't exist, it is created. The original file is left untouched.
if [ ! -z "$IMAGE" ]; then
    export VARS_ORIGINAL="$VARS"
    export VARS=`mktemp`
    cat "$VARS_ORIGINAL" > "$VARS"
    if [ -z "$VARS_ORIGINAL" ]; then
        echo "---" > $VARS
    fi

    yq w --inplace $VARS image "$IMAGE"
fi

export ACTIONS="true"

/app/deploy
