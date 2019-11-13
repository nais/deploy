#!/bin/sh

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

if [ ! -z "$IMAGE" ]; then
    if [ -z "$VARS" ]; then
        export VARS=`mktemp`
    fi
    echo "image: $IMAGE" >> $VARS
fi

/app/deploy \
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
