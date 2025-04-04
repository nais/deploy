#!/bin/sh
# vi: se et:

if [ -n "$APIKEY" ]; then
    echo "::add-mask::$APIKEY"
fi

if [ -n "$ACTIONS_ID_TOKEN_REQUEST_URL" ]; then
    echo "::add-mask::$ACTIONS_ID_TOKEN_REQUEST_URL"
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
    DEPLOY_JSON=$(mktemp)
    wget https://storage.googleapis.com/github-deploy-data/"$GITHUB_REPOSITORY_OWNER.json" --output-document "$DEPLOY_JSON"
    WGET_EXIT_CODE=$?
    cat "$DEPLOY_JSON"

    echo
    echo ::endgroup::
    if [ $WGET_EXIT_CODE -eq 0 ]; then
	    DEPLOY_SERVER=$(jq --raw-output '.DEPLOY_SERVER' < "$DEPLOY_JSON")
	    export DEPLOY_SERVER
    fi
fi

# if no apikey is set, use use the id-token to get a jwt token for the deploy CLI
# This is a bug, the security level of our ci stuff is at the same level as an apikey here since we offer that
# in addition to federated workload identity

if [ -z "$APIKEY" ]; then
    if [ -z "$ACTIONS_ID_TOKEN_REQUEST_TOKEN" ] || [ -z "$ACTIONS_ID_TOKEN_REQUEST_URL" ]; then
        echo "::error ::Missing id-token permissions. This must be set either globally in the workflow, or for the specific job performing the deploy."
        echo "::error ::For more info see https://doc.nais.io/build/how-to/build-and-deploy and/or https://docs.github.com/en/actions/using-jobs/assigning-permissions-to-jobs"

        echo "Ensure that you grant the following permissions in your workflow:" >> $GITHUB_STEP_SUMMARY
        echo '```yaml' >> $GITHUB_STEP_SUMMARY
        echo "permissions:" >> $GITHUB_STEP_SUMMARY
        echo "   id-token: write" >> $GITHUB_STEP_SUMMARY
        echo '```' >> $GITHUB_STEP_SUMMARY

        exit 1
    fi

    export GITHUB_TOKEN_URL="$ACTIONS_ID_TOKEN_REQUEST_URL"
    echo "::add-mask::$GITHUB_TOKEN_URL"
    export GITHUB_BEARER_TOKEN="$ACTIONS_ID_TOKEN_REQUEST_TOKEN"
    echo "::add-mask::$GITHUB_BEARER_TOKEN"
else
    echo "::warning ::APIKEY is deprecated. Update your workflow as per https://doc.nais.io/build/how-to/build-and-deploy"
fi

export ACTIONS="true"

# All of our users live in Norway, so why not. GitHub defaults to UTC.
export TZ="Europe/Oslo"

/app/deploy
