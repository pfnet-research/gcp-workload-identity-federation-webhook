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
)
