package webhooks

import (
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

// Volumes
var (
	// Volumes

	gcloudConfigVolume = corev1.Volume{
		Name: GCloudConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
)

func volumesToAddOrReplace(
	audience string,
	expirationSeconds int64,
	defaultMode int32,
) []corev1.Volume {
	return []corev1.Volume{k8sSATokenVolume(audience, expirationSeconds, defaultMode), gcloudConfigVolume}
}

func k8sSATokenVolume(
	audience string,
	expirationSeconds int64,
	defaultMode int32,
) corev1.Volume {
	return corev1.Volume{
		Name: K8sSATokenVolumeName,
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{{
					ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
						Audience:          audience,
						ExpirationSeconds: &expirationSeconds,
						Path:              K8sSATokenName,
					},
				}},
				DefaultMode: pointer.Int32(defaultMode),
			},
		},
	}
}

// Containers
func gcloudSetupContainer(
	workloadIdProvider, saEmail, project, gcloudImage string,
	runAsUser *int64,
	resources *corev1.ResourceRequirements,
) corev1.Container {
	// for Restricted Profile in Pod Security Standards
	securityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: pointer.Bool(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
	}
	if runAsUser != nil {
		securityContext = &corev1.SecurityContext{
			RunAsUser: runAsUser,
		}
	}

	c := corev1.Container{
		Name:  GCloudSetupInitContainerName,
		Image: gcloudImage,
		Command: []string{
			"sh", "-c",
			heredoc.Docf(`
				gcloud iam workload-identity-pools create-cred-config \
				  $(GCP_WORKLOAD_IDENTITY_PROVIDER) \
				  --service-account=$(GCP_SERVICE_ACCOUNT) \
				  --output-file=$(CLOUDSDK_CONFIG)/federation.json \
				  --credential-source-file=%s
				gcloud auth login --cred-file=$(CLOUDSDK_CONFIG)/federation.json
			`, filepath.Join(K8sSATokenMountPath, K8sSATokenName)),
		},
		VolumeMounts: volumeMountsToAddOrReplace,
		Env: []corev1.EnvVar{{
			Name:  "GCP_WORKLOAD_IDENTITY_PROVIDER",
			Value: workloadIdProvider,
		}, {
			Name:  "GCP_SERVICE_ACCOUNT",
			Value: saEmail,
		}, {
			Name:  "CLOUDSDK_CONFIG",
			Value: GCloudConifgMountPath,
		}, projectEnvVar(project)},
		SecurityContext: securityContext,
	}
	if resources != nil {
		c.Resources = *resources
	}
	return c
}

// VolumeMounts
var (
	volumeMountsToAddOrReplace = []corev1.VolumeMount{k8sSATokenVolumeMount, gcloudConfigVolumeMount}

	k8sSATokenVolumeMount = corev1.VolumeMount{
		Name:      K8sSATokenVolumeName,
		MountPath: K8sSATokenMountPath,
		ReadOnly:  true,
	}
	gcloudConfigVolumeMount = corev1.VolumeMount{
		Name:      GCloudConfigVolumeName,
		MountPath: GCloudConifgMountPath,
	}
)

// EnvVars
var (
	envVarsToAddOrReplace = []corev1.EnvVar{googleAppCredentialsEnvVar, cloudSDKConfigEnvVar}

	googleAppCredentialsEnvVar = corev1.EnvVar{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: filepath.Join(GCloudConifgMountPath, "federation.json"),
	}
	cloudSDKConfigEnvVar = corev1.EnvVar{
		Name:  "CLOUDSDK_CONFIG",
		Value: GCloudConifgMountPath,
	}
)

func envVarsToAddIfNotPresent(region, project string) []corev1.EnvVar {
	return []corev1.EnvVar{cloudSDKComputeRegionEnvVar(region), projectEnvVar(project)}
}

func cloudSDKComputeRegionEnvVar(region string) corev1.EnvVar {
	return corev1.EnvVar{
		Name:  "CLOUDSDK_COMPUTE_REGION",
		Value: region,
	}
}

func projectEnvVar(project string) corev1.EnvVar {
	return corev1.EnvVar{
		Name:  "CLOUDSDK_CORE_PROJECT",
		Value: project,
	}
}
