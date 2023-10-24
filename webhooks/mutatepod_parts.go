package webhooks

import (
	"fmt"
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

func (m *GCPWorkloadIdentityMutator) volumesToAddOrReplace(
	audience string,
	expirationSeconds int64,
	defaultMode int32,
	mode InjectionMode,
) []corev1.Volume {
	vols := []corev1.Volume{k8sSATokenVolume(audience, expirationSeconds, defaultMode)}

	if mode == DirectMode {
		vols = append(vols, m.externalCredConfigVolume(defaultMode))
	} else {
		vols = append(vols, gcloudConfigVolume)
	}

	return vols
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

func (m *GCPWorkloadIdentityMutator) externalCredConfigVolume(defaultMode int32) corev1.Volume {
	annoKey := fmt.Sprintf("%s/%s", m.AnnotationDomain, ExternalCredentialsJsonAnnotation)
	return corev1.Volume{
		Name: DirectInjectedExternalVolumeName,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: ExternalCredConfigFilename,
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  fmt.Sprintf("metadata.annotations['%s']", annoKey),
						},
					},
				},
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
		securityContext.RunAsUser = runAsUser
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
				  --output-file=$(CLOUDSDK_CONFIG)/%s \
				  --credential-source-file=%s
				gcloud auth login --cred-file=$(CLOUDSDK_CONFIG)/%s
			`, filepath.Join(K8sSATokenMountPath, K8sSATokenName),
				ExternalCredConfigFilename,
				ExternalCredConfigFilename,
			),
		},
		VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
		Env: []corev1.EnvVar{{
			Name:  "GCP_WORKLOAD_IDENTITY_PROVIDER",
			Value: workloadIdProvider,
		}, {
			Name:  "GCP_SERVICE_ACCOUNT",
			Value: saEmail,
		}, {
			Name:  "CLOUDSDK_CONFIG",
			Value: GCloudConfigMountPath,
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
	externalCredConfigVolumeMount = corev1.VolumeMount{
		Name:      DirectInjectedExternalVolumeName,
		MountPath: GCloudConfigMountPath,
		ReadOnly:  true,
	}
	k8sSATokenVolumeMount = corev1.VolumeMount{
		Name:      K8sSATokenVolumeName,
		MountPath: K8sSATokenMountPath,
		ReadOnly:  true,
	}
	gcloudConfigVolumeMount = corev1.VolumeMount{
		Name:      GCloudConfigVolumeName,
		MountPath: GCloudConfigMountPath,
	}
)

func volumeMountsToAddOrReplace(mode InjectionMode) []corev1.VolumeMount {
	volMounts := []corev1.VolumeMount{k8sSATokenVolumeMount}

	if mode == DirectMode {
		volMounts = append(volMounts, externalCredConfigVolumeMount)
	} else {
		volMounts = append(volMounts, gcloudConfigVolumeMount)
	}

	return volMounts
}

// EnvVars
var (
	envVarsToAddOrReplace = []corev1.EnvVar{googleAppCredentialsEnvVar, cloudSDKConfigEnvVar}

	googleAppCredentialsEnvVar = corev1.EnvVar{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: filepath.Join(GCloudConfigMountPath, ExternalCredConfigFilename),
	}
	cloudSDKConfigEnvVar = corev1.EnvVar{
		Name:  "CLOUDSDK_CONFIG",
		Value: GCloudConfigMountPath,
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
