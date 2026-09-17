package webhooks

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestGcloudSetupContainer(t *testing.T) {
	const (
		workloadIdProvider = "projects/12345/locations/global/workloadIdentityPools/on-prem-kubernetes/providers/this-cluster"
		saEmail            = "app-x@project.iam.googleapis.com"
		project            = "project"
		gcloudImage        = "gcr.io/google.com/cloudsdktool/google-cloud-cli:stable"
	)

	expectedTemplate := corev1.Container{
		Name:  "gcloud-setup",
		Image: gcloudImage,
		Command: []string{
			"sh", "-c",
			`gcloud iam workload-identity-pools create-cred-config \
  $(GCP_WORKLOAD_IDENTITY_PROVIDER) \
  --service-account=$(GCP_SERVICE_ACCOUNT) \
  --output-file=$(CLOUDSDK_CONFIG)/federation.json \
  --credential-source-file=/var/run/secrets/sts.googleapis.com/serviceaccount/token
gcloud auth login --cred-file=$(CLOUDSDK_CONFIG)/federation.json
`,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "gcp-iam-token",
				MountPath: "/var/run/secrets/sts.googleapis.com/serviceaccount",
				ReadOnly:  true,
			},
			{
				Name:      "gcloud-config",
				MountPath: "/var/run/secrets/gcloud/config",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "GCP_WORKLOAD_IDENTITY_PROVIDER",
				Value: workloadIdProvider,
			},
			{
				Name:  "GCP_SERVICE_ACCOUNT",
				Value: saEmail,
			},
			{
				Name:  "CLOUDSDK_CONFIG",
				Value: "/var/run/secrets/gcloud/config",
			},
			{
				Name:  "CLOUDSDK_CORE_PROJECT",
				Value: project,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
	}

	t.Run("Without runAsUser and resources", func(t *testing.T) {
		actual := gcloudSetupContainer(workloadIdProvider, saEmail, project, gcloudImage, nil, nil)
		expected := *expectedTemplate.DeepCopy()
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("gcloudSetupContainer() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("With runAsUser", func(t *testing.T) {
		user := int64(1000)
		actual := gcloudSetupContainer(workloadIdProvider, saEmail, project, gcloudImage, ptr.To(user), nil)

		expected := *expectedTemplate.DeepCopy()
		expected.SecurityContext.RunAsUser = ptr.To(user)

		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("gcloudSetupContainer() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("With resources", func(t *testing.T) {
		resources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		}
		actual := gcloudSetupContainer(workloadIdProvider, saEmail, project, gcloudImage, nil, &resources)

		expected := *expectedTemplate.DeepCopy()
		expected.Resources = resources

		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("gcloudSetupContainer() mismatch (-want +got):\n%s", diff)
		}
	})
}

// These pin down literal values so a future change to the DirectMode branch
// (e.g. dropping the gcloud CLI compatibility fields again) is caught even
// though callers in the rest of the package build their "expected" pods by
// calling these same functions.
func TestVolumeMountsToAddOrReplace(t *testing.T) {
	t.Run("GCloudMode", func(t *testing.T) {
		actual := volumeMountsToAddOrReplace(GCloudMode)
		expected := []corev1.VolumeMount{
			{Name: "gcp-iam-token", MountPath: "/var/run/secrets/sts.googleapis.com/serviceaccount", ReadOnly: true},
			{Name: "gcloud-config", MountPath: "/var/run/secrets/gcloud/config"},
		}
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("volumeMountsToAddOrReplace(GCloudMode) mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("DirectMode", func(t *testing.T) {
		actual := volumeMountsToAddOrReplace(DirectMode)
		expected := []corev1.VolumeMount{
			{Name: "gcp-iam-token", MountPath: "/var/run/secrets/sts.googleapis.com/serviceaccount", ReadOnly: true},
			{Name: "external-credential-config", MountPath: "/var/run/secrets/workload-identity", ReadOnly: true},
			{Name: "gcloud-config", MountPath: "/var/run/secrets/gcloud/config"},
		}
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("volumeMountsToAddOrReplace(DirectMode) mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestEnvVarsToAddOrReplace(t *testing.T) {
	t.Run("GCloudMode", func(t *testing.T) {
		actual := envVarsToAddOrReplace(GCloudMode)
		expected := []corev1.EnvVar{
			{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/run/secrets/gcloud/config/federation.json"},
			{Name: "CLOUDSDK_CONFIG", Value: "/var/run/secrets/gcloud/config"},
		}
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("envVarsToAddOrReplace(GCloudMode) mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("DirectMode", func(t *testing.T) {
		actual := envVarsToAddOrReplace(DirectMode)
		expected := []corev1.EnvVar{
			{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/run/secrets/workload-identity/federation.json"},
			{Name: "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", Value: "/var/run/secrets/workload-identity/federation.json"},
			{Name: "CLOUDSDK_CONFIG", Value: "/var/run/secrets/gcloud/config"},
		}
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("envVarsToAddOrReplace(DirectMode) mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestVolumesToAddOrReplace(t *testing.T) {
	m := &GCPWorkloadIdentityMutator{AnnotationDomain: AnnotationDomainDefault}
	const audience = "test-audience"
	const expirationSeconds int64 = 3600
	const defaultMode int32 = 0440

	t.Run("GCloudMode", func(t *testing.T) {
		actual := m.volumesToAddOrReplace(audience, expirationSeconds, defaultMode, GCloudMode)
		expected := []corev1.Volume{
			k8sSATokenVolume(audience, expirationSeconds, defaultMode),
			gcloudConfigVolume,
		}
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("volumesToAddOrReplace(GCloudMode) mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("DirectMode", func(t *testing.T) {
		actual := m.volumesToAddOrReplace(audience, expirationSeconds, defaultMode, DirectMode)
		expected := []corev1.Volume{
			k8sSATokenVolume(audience, expirationSeconds, defaultMode),
			m.externalCredConfigVolume(defaultMode),
			gcloudConfigVolume,
		}
		if diff := cmp.Diff(actual, expected); diff != "" {
			t.Errorf("volumesToAddOrReplace(DirectMode) mismatch (-want +got):\n%s", diff)
		}
	})
}
