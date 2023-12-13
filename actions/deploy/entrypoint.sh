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


# if no apikey is set, use use the id-token to get a jwt token for the deploy CLI
if [ -z "$APIKEY" ]; then
    if [ -z "$ACTIONS_ID_TOKEN_REQUEST_TOKEN" ] && [ -z "$ACTIONS_ID_TOKEN_REQUEST_URL" ]; then
        echo "APIKEY or id-token permissions must be set"
        exit 1
    fi

    payload=$(curl -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=hookd")
    jwt=$(echo "$payload" | jq -r '.value')

    export JWT="$jwt"
fi

export ACTIONS="true"


/app/deploy
