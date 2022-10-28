# GCP Workload Identity Federation Webhook

This webhook is for mutating pods that will require GCP Workload Identity Federation access from Kubernetes Cluster.

Note: GKE or Anthos natively support injecting workload identity for pods.  This webhook is useful mainly for Kubernetes clusters running in other cloud providers or on-premise.

## Prerequisites

1. Kubernetes cluster v1.21 or later.

2. Configure kube-apiserver with `--service-account-issuer` and `--service-account-jwks-uri` properly.

3. Expose [Your Kubernetes Cluster's ServiceAccount issuer discovery endpoint (OIDC discovery endpoint)](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-issuer-discovery) to the public so that it can reach by the url set in `--service-account-issuer`
  - _Hint: you can use S3/GCS to expose the endpoint to the public._
  - __WARNING: If your public JWKS(JSON Web Key Set) endpoint are compromised by the malicious attacker, the attacker can hijack your JWKS endpoint (i.e., issue OIDC ID Tokens) that can impersonate any roles for GCP service accounts that are configured to trust to the issuer. Thus, JWKS endpoint must be secured all the time.__

## Walk Through

1. [Create an external identity pool and provider][wif] with your OIDC issuer and allowed audience (default: `sts.googleapis.com`) in IAM for your project. It will need to configure attribute mappings and conditions from OIDC ID Tokens to identities in the pool.

2. [Granting external identities permission to impersonate a service account][grant-sa] so that a Kubernetes `ServiceAccount` can work as a _federated_ workload identity (i.e., it can impersonate a GCP service account).

3. Annotate a Kubernetes `ServiceAccount` with the identity provider and GCP service account that it will impersonate.

    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: app-x
      namespace: service-a
      annotations:
        # assume 
        #   you grant k8s service account "service-a/app-x" to impersonate "app-x" GCP service account
        #   this k8s cluster's service account belongs the annotated workload identity provider here
        cloud.google.com/workload-identity-provider: "projects/12345/locations/global/workloadIdentityPools/on-prem-kubernetes/providers/this-cluster"
        cloud.google.com/service-account-email: "app-x@project.iam.googleapis.com"

        # optional: Defaults to "sts.googleapis.com" if not set
        #   this value must be allowed in the annotated workload identity provider above
        cloud.google.com/audience: "sts.googleapis.com"

        # optional: Defaults to 86400 for expirationSeconds if not set
        #   Note: This value can be overwritten if specified in the pod 
        #         annotation as shown in the next step.
        cloud.google.com/token-expiration: "86400"
    ```

4. All new pods launched using the Kubernetes `ServiceAccount` will be mutated so that they can impersonate the GCP service account. Below is an example pod spec with the environment variables and volume fields mutated by the webhook.

    ```yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: app-x-pod
      namespace: service-a
    annotations:
      # optional: A comma-separated list of initContainers and container names
      #   to skip adding volumeMounts and environment variables
      cloud.google.com/skip-containers: "init-first,sidecar"
      # optional: Defaults to 86400, or value specified in ServiceAccount
      #   annotation as shown in previous step, for expirationSeconds if not set
      cloud.google.com/token-expiration: "86400"
    spec:
      serviceAccountName: app-x
      initContainers:
        ### gcloud-setup init container is injected by the webhook ###
      - name: gcloud-setup
        image: google/cloud-sdk:slim
        command:
        - sh
        - -c
        - |
          gcloud iam workload-identity-pools create-cred-config \
              $(GCP_WORKLOAD_IDENTITY_PROVIDER) \
              --service-account=$(GCP_SERVICE_ACCOUNT) \
              --output-file=$(CLOUDSDK_CONFIG)/federation.json \
              --credential-source-file=/var/run/secrets/sts.googleapis.com/serviceaccount/token
              gcloud auth login --cred-file=$(CLOUDSDK_CONFIG)/federation.json
        env:
        - name: GCP_WORKLOAD_IDENTITY_PROVIDER
          value: "projects/12345/locations/global/workloadIdentityPools/on-prem-kubernetes/providers/this-cluster"
        - name: GCP_SERVICE_ACCOUNT
          value: app-x@project.iam.gserviceaccount.com
        - name: CLOUDSDK_CONFIG
          value: /var/run/secrets/gcloud/config
        volumeMounts:
        - name: gcp-iam-token
          readOnly: true
          mountPath: /var/run/secrets/sts.googleapis.com/serviceaccount
        - name: gcloud-config
          mountPath: /var/run/secrets/gcloud/config
      - name: init-first
        image: container-image:version
      containers:
      - name: sidecar
        image: container-image:version
      - name: container-name
        image: container-image:version
        ### Everything below is added by the webhook ###
        env:
        - name: GOOGLE_APPLICATION_CREDENTIALS
          value: /var/run/secrets/gcloud/config/federation.json
        - name: CLOUDSDK_CONFIG
          value: /var/run/secrets/gcloud/config
        - name: CLOUDSDK_COMPUTE_REGION
          value: asia-northeast1
        volumeMounts:
        - name: gcp-iam-token
          readOnly: true
          mountPath: /var/run/secrets/sts.googleapis.com/serviceaccount
        - name: gcloud-config
          mountPath: /var/run/secrets/gcloud/config
      volumes:
      - name: gcp-iam-token
        projected:
          sources:
          - serviceAccountToken:
              audience: sts.googleapis.com
              expirationSeconds: 86400
              path: token
      - name: gcloud-config
        emptyDir: {}
    ```

[wif]: https://cloud.google.com/iam/docs/configuring-workload-identity-federation#oidc
[grant-sa]: https://cloud.google.com/iam/docs/using-workload-identity-federation#impersonate

### Usage with non-root container user

When running a container with a non-root user, you need to give the container access to the token file by setting the `securityContext.fsGroup` field.

## Usage

```
Usage of /gcp-workload-identity-federation-webhook:
  -annotation-prefix string
        The Service Account annotation to look for (default "cloud.google.com")
  -gcloud-image string
        Container image for the init container setting up GCloud SDK (default "gcloud:slim")
  -gcp-default-region string
        If set, CLOUDSDK_COMPUTE_REGION will be set to this value in mutated containers
  -health-probe-bind-address string
        The address the probe endpoint binds to. (default ":8081")
  -kubeconfig string
        Paths to a kubeconfig. Only required if out-of-cluster.
  -metrics-bind-address string
        The address the metric endpoint binds to. (default ":8080")
  -token-audience string
        The default audience for tokens. Can be overridden by annotation (default "sts.googleapis.com")
  -token-expiration duration
        The token expiration (default 24h0m0s)
  -zap-devel
        Development Mode defaults(encoder=consoleEncoder,logLevel=Debug,stackTraceLevel=Warn). Production Mode defaults(encoder=jsonEncoder,logLevel=Info,stackTraceLevel=Error) (default true)
  -zap-encoder value
        Zap log encoding (one of 'json' or 'console')
  -zap-log-level value
        Zap Level to configure the verbosity of logging. Can be one of 'debug', 'info', 'error', or any integer value > 0 which corresponds to custom debug levels of increasing verbosity
  -zap-stacktrace-level value
        Zap Level at and above which stacktraces are captured (one of 'info', 'error', 'panic').
  -zap-time-encoding value
        Zap time encoding (one of 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'). Defaults to 'epoch'.
```

## Installation

### Pre-requisites

- cert-manager: See [cert-manager installation](https://cert-manager.io/docs/installation/)
- (optional) prometheus-operator: See https://github.com/prometheus-operator/prometheus-operator

### Deploy

```shell
make deploy
```

Or, please inspect `config/default` directory.

## Release

The release process is automated by [tagpr](https://github.com/Songmu/tagpr). To release, just merge [the latest release PR](https://github.com/pfnet-research/gcp-workload-identity-federation-webhook/pulls?q=is:pr+is:open+label:tagpr).

## License

Apache 2.0 - Copyright 2022 Preferred Networks, Inc. or its affiliates. All Rights Reserved.
See [LICENSE](LICENSE)

## Acknowledgment

This project is greatly inspired by [aws/amazon-eks-pod-identity-webhook](https://github.com/aws/amazon-eks-pod-identity-webhook).
