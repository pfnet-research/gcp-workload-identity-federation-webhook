package webhooks

const (
	//
	// Annotations for ServiceAccount
	//
	// The workloadIdentityProvider annotattion
	// This must be the format of "projects/{PROJECT_NUMBER}/locations/{LOCATION}/workloadIdentityPools/{POOL_ID}/providers/{PROVIDER_ID}"
	WorkloadIdentityProviderAnnotation = "workload-identity-provider"

	// The serviceaccount email annotation
	ServiceAccountEmailAnnotation = "service-account-email"

	// The audience annotation
	AudienceAnnotation = "audience"

	//
	// Annotations for ServiceAccount
	//
	// UserID to be set in the container securityContext.runAsUser for the gcloud sdk
	RunAsUserAnnotation = "gcloud-run-as-user"

	//
	// Annotations for both ServiceAccount and Pod
	//
	// TokenExpiration annotation in seconds
	TokenExpirationAnnotation = "token-expiration"

	//
	// Annotations for Pod
	//
	// A comma-separated list of container names to skip adding environment variables and volumes to. Applies to `initContainers` and `containers`
	SkipContainersAnnotation = "skip-containers"

	//
	// Annotations for Pod
	//
	// The External Credentials JSON blob to be injected into the cluster, only used in 'direct' mode.
	ExternalCredentialsJsonAnnotation = "external-credentials-json"

	//
	// Annotations for ServiceAccount
	//
	// Set to 'direct' or 'gcloud' to determine credential injection mode. Defaults to 'gcloud'.
	InjectionModeAnnotation = "injection-mode"

	//
	// Annotations for ServiceAccount
	//
	// Set to 'service-account' or 'direct-access' to determine the GCP token exchange mode.
	// Defaults to 'service-account'. In 'direct-access' mode, the ServiceAccountEmailAnnotation
	// is ignored and the exchanged STS token is used to access GCP resources directly as the
	// federated principal (no impersonation).
	TokenExchangeModeAnnotation = "token-exchange-mode"

	//
	// Annotations for ServiceAccount
	//
	// Optional GCP project ID, used to populate CLOUDSDK_CORE_PROJECT in mutated pods.
	// When set, overrides the project ID extracted from ServiceAccountEmailAnnotation.
	// Useful in 'direct-access' token-exchange-mode where no service-account-email is
	// available, or when the workload should target a project other than the one in the
	// service account email.
	ProjectIDAnnotation = "project-id"
)
