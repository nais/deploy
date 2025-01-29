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

if [ -z "$DEPLOY_SERVER" ]; then
    echo ::group::wget
    wget https://storage.googleapis.com/github-deploy-data/$GITHUB_REPOSITORY_OWNER.json --output-document deploy.json
    cat deploy.json

    #this is a newline!
    echo
    echo ::endgroup::
    export DEPLOY_SERVER=$(jq --raw-output '.DEPLOY_SERVER' < deploy.json)
fi

# if no apikey is set, use use the id-token to get a jwt token for the deploy CLI
# This is a bug, the security level of our ci stuff is at the same level as an apikey here since we offer that
# in addition to federated workload identity
if [ -z "$APIKEY" ]; then
    if [ -z "$ACTIONS_ID_TOKEN_REQUEST_TOKEN" ] || [ -z "$ACTIONS_ID_TOKEN_REQUEST_URL" ]; then
        echo "Missing id-token permissions. This must be set either globally in the workflow, or for the specific job performing the deploy."
        echo "For more info see https://doc.nais.io/build/how-to/build-and-deploy and/or https://docs.github.com/en/actions/using-jobs/assigning-permissions-to-jobs"

        exit 1
    fi

    payload=$(curl -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=hookd")
    jwt=$(echo "$payload" | jq -r '.value')

    export GITHUB_TOKEN="$jwt"
fi

export ACTIONS="true"

# All of our users live in Norway, so why not. GitHub defaults to UTC.
export TZ="Europe/Oslo"

/app/deploy
