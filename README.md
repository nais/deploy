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

The usage documentation has been moved to [NAIS platform documentation](https://github.com/nais/doc/tree/master/content/deploy).

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

### Development on the frontend application
To enable the frontend on your local instance, you need to configure hookd against the staging deployment application at Github.
This is required to enable OAuth and Github queries.
The parameters can be found on the [Github installation page](https://github.com/organizations/navikt/settings/installations/).
You must also generate a private key for this installation, in order to sign your JSON web tokens.

Configure hookd as follows:

```
--github-enabled=true \
--github-install-id=XXXXXX \
--github-app-id=XXXXXX \
--github-key-file=/path/to/private-key.pem \
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
