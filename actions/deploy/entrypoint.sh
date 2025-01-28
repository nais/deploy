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

if [ -n "$DEPLOY_SERVER" ]; then
  echo "::notice ::DEPLOY_SERVER is deprecated and should not be set, please remove from your workflow"
fi

echo ::group::wget
WGET_OUTPUT=$(wget -q -O - https://storage.googleapis.com/github-deploy-data/$GITHUB_REPOSITORY_OWNER.json 2>/dev/null)
WGET_EXIT_CODE=$?

# If wget fails, then we fallthrough and assume that we are test-nais (all ohter GITHUB_REPOSITYR_OWNERS have a 1-1 with a var.tenant_name in terraform)
# This is because test-nais uses some other github_repository_owner (per org that is testing!) that we don't know about
if [ $WGET_EXIT_CODE -ne 0 ]; then
    echo "failed getting deploy_data, using fallback (you are now in test-nais!)"
    WGET_OUTPUT=$(wget -q -O - https://storage.googleapis.com/github-deploy-data/test-nais.json 2>/dev/null)
fi

echo "$WGET_OUTPUT" > deploy.json
cat deploy.json

#this is a newline to close the wget group!
echo
echo ::endgroup::

export DEPLOY_SERVER=$(jq --raw-output '.DEPLOY_SERVER' < deploy.json)

# if no apikey is set, use use the id-token to get a jwt token for the deploy CLI
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
