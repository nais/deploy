#!/bin/sh
# vi: se et:

if [ -n "$APIKEY" ]; then
    echo "::add-mask::$APIKEY"
fi
if [ -n "$ACTIONS_ID_TOKEN_REQUEST_TOKEN" ]; then
    echo "::add-mask::$ACTIONS_ID_TOKEN_REQUEST_TOKEN"
fi

if [ -z "$OWNER" ]; then
    OWNER=$(echo "$GITHUB_REPOSITORY" | cut -f1 -d/)
    export OWNER
fi

if [ -z "$REPOSITORY" ]; then
    REPOSITORY=$(echo "$GITHUB_REPOSITORY" | cut -f2 -d/)
    export REPOSITORY
fi

if [ -z "$REF" ]; then
    export REF="$GITHUB_REF"
fi

if [ -z "$WAIT" ]; then
    export WAIT="true"
fi

# Inject "image" as a template variable to a new copy of the vars file.
# If the file doesn't exist, it is created. The original file is left untouched.
if [ -n "$IMAGE" ]; then
    export VARS_ORIGINAL="$VARS"
    VARS=$(mktemp)
    export VARS
    if [ -z "$VARS_ORIGINAL" ]; then
        echo "---" > "$VARS"
    else
        cat "$VARS_ORIGINAL" > "$VARS"
    fi
    yq w --inplace "$VARS" image "$IMAGE"
fi


# if no apikey is set, use use the id-token to get a jwt token for the deploy CLI
if [ -z "$APIKEY" ]; then
    if [ -z "$ACTIONS_ID_TOKEN_REQUEST_TOKEN" ] || [ -z "$ACTIONS_ID_TOKEN_REQUEST_URL" ]; then
        echo "Missing id-token permissions. This must be set either globally in the workflow, or for the specific job performing the deploy."
        echo "For more info see https://doc.nais.io/deployment/github-action/ and/or https://docs.github.com/en/actions/using-jobs/assigning-permissions-to-jobs"

        exit 1
    fi

    payload=$(curl -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=hookd")
    jwt=$(echo "$payload" | jq -r '.value')

    export GITHUB_TOKEN="$jwt"
fi

export ACTIONS="true"

/app/deploy
