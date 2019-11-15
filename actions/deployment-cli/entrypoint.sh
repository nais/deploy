#!/bin/sh -l

/deployment-cli deploy create --cluster=$INPUT_CLUSTER --team=$INPUT_TEAM --repository=$INPUT_REPOSITORY --resource=$GITHUB_WORKSPACE/$INPUT_RESOURCE --vars=$GITHUB_WORKSPACE/$INPUT_VARS --token=$GITHUB_TOKEN
