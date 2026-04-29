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

        # optional: This value defines the container security context with runAsUser
        #           with the defined user. This could avoid problems related with root requirement from gcloud image
        cloud.google.com/gcloud-run-as-user: "1000"

        # optional: gcloud external configuration injection mode.
        #           The value must be one of 'gcloud'(default) or 'direct'.
        #           Refer to the next section for 'direct' injection mode
        cloud.google.com/injection-mode: "gcloud"

        # optional: Token exchange mode. Value must be one of 'service-account' (default) or 'direct-access'.
        #           Refer to the "Direct Resource Access" section for 'direct-access' mode.
        cloud.google.com/token-exchange-mode: "service-account"

        # optional: GCP project ID injected as CLOUDSDK_CORE_PROJECT in mutated pods.
        #           When unset, the webhook extracts the project ID from
        #           cloud.google.com/service-account-email. Set this explicitly to override,
        #           or when no SA email is available (e.g. in 'direct-access' mode).
        cloud.google.com/project-id: "project"
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
        image: gcr.io/google.com/cloudsdktool/google-cloud-cli:stable
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

When running a container with a non-root user, you need to give user id for GCloud SDK container using the annotation `cloud.google.com/gcloud-run-as-user` in the service account.

## Token Exchange Modes

Select via the ServiceAccount annotation `cloud.google.com/token-exchange-mode`.

### Service Account Impersonation (default)

The value `service-account` (or an absent annotation) preserves existing behavior: the exchanged STS token impersonates the GCP service account in `cloud.google.com/service-account-email`. See the "Walk Through" section above for setup.

### Direct Resource Access

In `direct-access` mode the webhook configures the workload to access GCP resources directly as the federated principal, with no intermediate service account. This removes the service-account management step but requires IAM bindings that reference the federated principal directly.

1. Annotate the Kubernetes ServiceAccount:

    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: app-x
      namespace: service-a
      annotations:
        cloud.google.com/workload-identity-provider: "projects/12345/locations/global/workloadIdentityPools/on-prem-kubernetes/providers/this-cluster"
        cloud.google.com/token-exchange-mode: "direct-access"
        # cloud.google.com/service-account-email is not required (it is ignored if present)
        # optional: project ID injected as CLOUDSDK_CORE_PROJECT for gcloud / Application Default Credentials.
        # Without this, direct-access mode cannot derive a project ID (the workload-identity-provider only
        # exposes the project number).
        cloud.google.com/project-id: "my-project"
    ```

2. Grant IAM bindings on the resources you want the workload to access. Bind the `principal://` (single subject) or `principalSet://` (attribute set) member, not a service account:

    ```
    # example: grant Storage Object Viewer on a bucket to a single subject
    gcloud storage buckets add-iam-policy-binding gs://my-bucket \
      --member='principal://iam.googleapis.com/projects/PROJECT_NUMBER/locations/global/workloadIdentityPools/on-prem-kubernetes/subject/SUBJECT_VALUE' \
      --role=roles/storage.objectViewer
    ```

    Replace `SUBJECT_VALUE` with whatever value is produced by the `google.subject` attribute mapping in your Workload Identity Pool Provider.

    For an attribute-based binding, use `principalSet://...workloadIdentityPools/POOL_ID/attribute.NAME/VALUE`.

    See the [Google Cloud documentation on impersonation vs. direct resource access][impersonation-vs-direct] for details and the [list of products that support direct resource access][direct-support].

3. The resulting pod spec is similar to the "Direct" injection example below, except the `cloud.google.com/external-credentials-json` annotation does not contain a `service_account_impersonation_url` field. Example:

    ```json
    {
      "type": "external_account",
      "audience": "//iam.googleapis.com/projects/12345/locations/global/workloadIdentityPools/on-prem-kubernetes/providers/this-cluster",
      "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
      "token_url": "https://sts.googleapis.com/v1/token",
      "credential_source": {
        "file": "/var/run/secrets/sts.googleapis.com/serviceaccount/token",
        "format": { "type": "text" }
      }
    }
    ```

[impersonation-vs-direct]: https://cloud.google.com/iam/docs/workload-identity-federation#impersonation_versus_direct_resource_access
[direct-support]: https://cloud.google.com/iam/docs/federated-identity-supported-services

## Credential Injection Modes

The webhook has two independent configuration axes for ServiceAccounts:

- **Token Exchange Mode** (see "Token Exchange Modes" above): what the exchanged STS token represents — a GCP Service Account (default) or the federated principal itself.
- **Credential Injection Mode** (this section): how the generated external credential configuration is delivered to the pod — via a `gcloud` init container (default) or directly through a DownwardAPI volume.

Defaults (`service-account` + `gcloud`) preserve behavior from earlier releases.

### Direct (Experimental)

In this mode, the Workload Identity Federation Webhook controller directly generates the Gcloud external credentials configuration and injects into the pod.
This means the `gcloud-setup` init container is not required which can speed up pod start time.

To use direct injection mode:

1. Annotate a Kubernetes `ServiceAccount` with the `injection-mode` value of `direct`.

    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: app-x
      namespace: service-a
      annotations:

        # Set the injection mode to 'direct', instead of 'gcloud'.
        cloud.google.com/injection-mode: "direct"
    ```

2. Below is an example pod spec with the environment variables and volume fields mutated by the webhook. Notice there is no `gcloud-setup` init container or Volumes, instead there is an extra annotation and `external-credential-config` volume and volumeMount.

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
      #
      # The Generated External Credentials Json is added as an annotation, and mounted into the container filesystem via the DownwardAPI Volume
      #
      cloud.google.com/external-credentials-json: |-
        {
          "type": "external_account",
          "audience": "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/on-prem-kubernetes/providers/this-cluster",
          "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
          "token_url": "https://sts.googleapis.com/v1/token",
          "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/app-x@project.iam.gserviceaccount.com:generateAccessToken",
          "credential_source": {
            "file": "/var/run/secrets/sts.googleapis.com/serviceaccount/token",
            "format": {
              "type": "text"
            }
          }
        }
    spec:
      serviceAccountName: app-x
      initContainers:
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
        - name: CLOUDSDK_COMPUTE_REGION
          value: asia-northeast1
        volumeMounts:
        - name: gcp-iam-token
          readOnly: true
          mountPath: /var/run/secrets/sts.googleapis.com/serviceaccount
        - mountPath: /var/run/secrets/gcloud/config
          name: external-credential-config
          readOnly: true
      volumes:
      - name: gcp-iam-token
        projected:
          sources:
          - serviceAccountToken:
              audience: sts.googleapis.com
              expirationSeconds: 86400
              path: token
      - downwardAPI:
          defaultMode: 288
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.annotations['cloud.google.com/external-credentials-json']
            path: federation.json
        name: external-credential-config
    ```

## Usage

```console
Usage of /gcp-workload-identity-federation-webhook:
  -annotation-prefix string
        The Service Account annotation to look for (default "cloud.google.com")
  -gcloud-image string
        Container image for the init container setting up GCloud SDK (default "gcr.io/google.com/cloudsdktool/google-cloud-cli:stable")
  -gcp-default-region string
        If set, CLOUDSDK_COMPUTE_REGION will be set to this value in mutated containers
  -health-probe-bind-address string
        The address the probe endpoint binds to. (default ":8081")
  -kubeconfig string
        Paths to a kubeconfig. Only required if out-of-cluster.
  -metrics-bind-address string
        The address the metric endpoint binds to. (default ":8080")
  -setup-container-resources string
        Resource spec in json for the init container setting up GCloud SDK, e.g. '{"requests":{"cpu":"100m"}}'
  -token-audience string
        The default audience for tokens. Can be overridden by annotation (default "sts.googleapis.com")
  -token-expiration duration
        The token expiration (default 24h0m0s)
  -token-default-mode int
        DefaultMode for the token volume (default 0440)
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

#### Helm chart

```shell
$ helm repo add gcp-workload-identity-federation-webhook https://pfnet-research.github.io/gcp-workload-identity-federation-webhook
$ helm repo update
$ helm install gcp-wif-webhook gcp-workload-identity-federation-webhook/gcp-workload-identity-federation-webhook \
    --namespace gcp-wif-webhook-system --create-namespace
```

#### Kustomize

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
