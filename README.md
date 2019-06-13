# NAIS deployment

## Overview
NAIS deployment facilitates application deployment into NAV's Kubernetes clusters using the Github Deployments API.

Developers push or merge code into the master branch of a Git repository, triggering an automated build using CircleCI, Travis CI, or Jenkins.
A successful build produces a Docker image artifact, which is uploaded onto Docker Hub.
The final step in the build pipeline triggers the Github Deployments API, where NAIS deployment hooks in, deploying the application on Kubernetes.

![Sequence diagram of deployment components](doc/sequence.png)

## How it works
1. As the final step in one of your CI pipelines, a [deployment request](https://developer.github.com/v3/repos/deployments/#create-a-deployment) is created using GitHub's API. This will trigger a webhook set up at Github.
2. `hookd` receives the webhook, verifies its integrity and authenticity, and passes the message on to `deployd` via Kafka.
3. `deployd` receives the message from `hookd`, assumes the identity of the deploying team, and applies your _Kubernetes resources_ into the specified [environment](#environment).
4. If the Kubernetes resources contained any _Application_ or _Deployment_ resources, `deployd` will wait until these are rolled out successfully, or a timeout occurs.

Any fatal error will short-circuit the process with a `error` or `failure` status posted back to Github. A successful deployment will result in a `success` status.
Intermediary statuses might be posted, indicating the current state of the deployment.

## Usage

### Prerequisites
* Your application must be [Naiserator compatible](https://github.com/nais/doc/tree/master/content/deploy). Deployment orchestration only acts on Kubernetes resources.
* Limit write access on your Github repository to team members. After activation, anyone with write access to the repository can deploy Kubernetes resources on behalf of your team.
* Be a maintainer of a [Github team](https://help.github.com/en/articles/about-teams). The team name must be the same as your Kubernetes _team label_.

### Registering your repository
You need to grant your Github repository access rights to deploy on behalf of your team.
In order to do this, you need to have _maintainer_ access rights to your Github team, and _admin_ access to the repository.

Visit the [registration portal](https://deployment.prod-sbs.nais.io/auth/login) and follow the instructions.

### Authenticating to the Github API
There are two ways to authenticate API requests: using a Github App, or with a personal access token.

The first option is unfortunately currently only available to Github organization admins. You can still self-service by
[creating a personal access token](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line)
that your CI pipeline can use to trigger the deployment. The token needs only the scope `repo_deployment`.

Usage from curl looks like this:

```
% curl -u USERNAME:TOKEN https://api.github.com/...
```

### Deployment requests
A deployment into the Kubernetes clusters starts with a POST request to the [GitHub Deployment API](https://developer.github.com/v3/repos/deployments/#create-a-deployment).
The request contains information about which environment to deploy to, which team to deploy as, and what resources should be applied.

#### Deploying using deployment-cli
Our internal tool [deployment-cli](https://github.com/navikt/deployment-cli) will enable you to make deployment requests in a way similar to:

```
deployment-cli create \
  --environment=dev-fss \
  --repository=navikt/deployment \
  --team=<TEAM> \
  --version=<VERSION> \
  --appid=1234 \
  --key=/path/to/private-key.pem \
  --resources=nais.yaml \
  --config=placeholders.json
```

#### Deploying using cURL
Example request:
```
{
    "ref": "master",
    "description": "Automated deployment request from our pretty pipeline",
    "environment": "prod-sbs",
    "payload": {
        "version": [1, 0, 0],
        "team": "github-team-name",
        "kubernetes": {
            "resources": [
                { kind: "Application", apiVersion: "nais.io/v1alpha", metadata: {...}, spec: {...} },
                { kind: "ConfigMap", apiVersion: "v1", metadata: {...}, spec: {...} },
            ],
        }
    }
}
```

The data can be posted from standard input through curl using a command similar to:

```
curl \
    -X POST \
    -d@- \
    -H "Accept: application/vnd.github.ant-man-preview+json" \
    -u <USERNAME>:<TOKEN> \
    https://api.github.com/repos/navikt/<REPOSITORY_NAME>/deployments
```

### Deployment request spec

| Key | Description | Version added |
|-----|-------------|---------------|
| environment | Which environment to deploy to. | N/A |
| payload.version | This is the *payload API version*, as described below. Array of three digits, denoting major, minor, and patch level version. | 1.0.0 |
| payload.team | Github team name, used as credentials for deploying into the Kubernetes cluster. | 1.0.0 |
| payload.kubernetes.resources | List of Kubernetes resources that should be applied into the cluster. Your `nais.yaml` file goes here, in JSON format instead of YAML. | 1.0.0 |

#### Environment
Please use one of the following environments. The usage of `preprod-***` is *not* supported.
  * `dev-fss`
  * `dev-sbs`
  * `prod-fss`
  * `prod-sbs`

#### Payload API versioning
When making API requests, please use the most recent version `[1, 0, 0]`.

This version field have nothing to do with your application version. It is used internally by the deployment orchestrator to
keep things stable and roll out new features gracefully.

Changes will be rolled out using [semantic versioning](https://semver.org).

### Troubleshooting
Generally speaking, if the deployment status is anything else than `success`, `queued`, or `pending`, it means that your deployment failed.

Please check the logs before asking. To get a link to your logs, please check the deployment status page at
`https://github.com/navikt/<YOUR_REPOSITORY>/deployments`.

If everything fails, report errors to #nais-deployment on Slack.

#### Common scenarios

| Message | Action |
|---------|--------|
| Repository _foo/bar_ is not registered | Please read the [registering your repository](#registering-your-repository) section. |
| Deployment status `error` | There is an error with your request. The reason should be specified in the error message. |
| Deployment status `failure` | Your application didn't pass its health checks during the 5 minute startup window. It is probably stuck in a crash loop due to mis-configuration. Check your application logs using `kubectl logs <POD>` and event logs using `kubectl describe app <APP>`
| Deployment is stuck at `queued` | The deployment hasn't been picked up by the worker process. Did you specify the [correct environment](#environment) in the `environment` variable? |


## Application components

### hookd
This service communicates with Github, and acts as a relay between the Internet and our Kubernetes clusters.

Its main tasks are to:
* validate deployment events
* relay deployment requests to _deployd_ using Kafka
* report deployment status back to GitHub

The validation part is done by checking if the signature attached to the deployment event is valid, and by checking the format of the deployment.
Refer to the [GitHub documentation](https://developer.github.com/webhooks/securing/) as to how webhooks are secured.

### deployd
Deployd responsibility is to deploy resources into a Kubernetes cluster, and report state changes back to hookd using Kafka.

### Kafka
Kafka is used as a communication channel between hookd and deployd. Hookd sends deployment requests to a `deploymentRequests` topic, which fans out
and in turn hits all the deployd instances. Deployd acts on the information, and then sends a deployment status to the `deploymentStatus` topic.
Hookd picks up replies to this topic, and publishes the deployment status to Github.

### Amazon S3 (Amazon Simple Storage Service)
Used as a configuration backend. Information about repository team access is stored here, and accessed on each deployment request.


## Developing

### Compiling hookd and deployd
[Install Golang 1.12 or newer](https://golang.org/doc/install).

Check out the repository and run `make`. Dependencies will download automatically, and you should have two binary files at `hookd/hookd` and `deployd/deployd`.

### External dependencies
Start the external dependencies by running `docker-compose up`. This will start local Kafka and S3 servers.

The S3 access and secret keys are as follows:

```
export S3_ACCESS_KEY=accesskey
export S3_SECRET_KEY=secretkey
```

### Simulating Github deployment requests
When you want to send webhooks to _hookd_ without invoking Github, you can use the `mkdeploy` tool, which simulates these requests.

Start a local Kafka instance as described above. Now run your local hookd instance, disabling Github interactions:
```
hookd/hookd --github-enabled=false --listen-address=127.0.0.1:8080
```

If you want to deploy, you want to start up `deployd` as well:
```
deployd/deployd
```

Compile the `mkdeploy` tool:
```
cd hookd/cmd/mkdeploy
make
```

You can now run `mkdeploy`. The default parameters should work fine, but you'll probably want to specify a deployment payload, which, by default, is empty.
Run `./mkdeploy --help` to see which options you can tweak.
