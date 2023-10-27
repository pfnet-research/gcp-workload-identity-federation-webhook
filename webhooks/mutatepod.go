package webhooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var projectRegex *regexp.Regexp

func init() {
	projectRegex = regexp.MustCompile(`@(.*).iam.gserviceaccount.com`)
}

func (m *GCPWorkloadIdentityMutator) mutatePod(pod *corev1.Pod, idConfig GCPWorkloadIdentityConfig) error {
	audience := m.DefaultAudience
	if idConfig.Audience != nil {
		audience = *idConfig.Audience
	}

	expirationSeconds := int64(m.DefaultTokenExpiration.Seconds())
	if idConfig.TokenExpirationSeconds != nil {
		expirationSeconds = *idConfig.TokenExpirationSeconds
	}
	if expRaw, ok := pod.Annotations[filepath.Join(m.AnnotationDomain, TokenExpirationAnnotation)]; ok {
		seconds, err := strconv.ParseInt(expRaw, 10, 64)
		if err != nil {
			return fmt.Errorf("%s must be positive integer string: %w", filepath.Join(m.AnnotationDomain, TokenExpirationAnnotation), err)
		}
		expirationSeconds = seconds
	}
	if expirationSeconds < int64(m.MinTokenExpration.Seconds()) {
		expirationSeconds = int64(m.MinTokenExpration.Seconds())
	}

	// mutate annotations
	if pod.Annotations == nil {
		pod.Annotations = map[string]string{}
	}
	pod.Annotations[filepath.Join(m.AnnotationDomain, WorkloadIdentityProviderAnnotation)] = *idConfig.WorkloadIdentityProvider
	pod.Annotations[filepath.Join(m.AnnotationDomain, ServiceAccountEmailAnnotation)] = *idConfig.ServiceAccountEmail
	pod.Annotations[filepath.Join(m.AnnotationDomain, AudienceAnnotation)] = audience
	pod.Annotations[filepath.Join(m.AnnotationDomain, TokenExpirationAnnotation)] = fmt.Sprint(expirationSeconds)
	if idConfig.InjectionMode == DirectMode {
		// Add annotation
		credBody, err := buildExternalCredentialsJson(*idConfig.WorkloadIdentityProvider, *idConfig.ServiceAccountEmail)
		if err != nil {
			return err
		}
		pod.Annotations[filepath.Join(m.AnnotationDomain, ExternalCredentialsJsonAnnotation)] = credBody
	}

	//
	// calculate project from service account
	//
	matches := projectRegex.FindStringSubmatch(*idConfig.ServiceAccountEmail)
	project := ""
	if len(matches) >= 2 {
		project = matches[1] // the group 0 is thw whole match
	}

	//
	// mutate volumes(k8s sa token volume, gcloud config volume)
	//
	for _, v := range m.volumesToAddOrReplace(audience, expirationSeconds, int32(m.DefaultMode), idConfig.InjectionMode) {
		pod.Spec.Volumes = addOrReplaceVolume(pod.Spec.Volumes, v)
	}

	//
	// inject gcloud setup initContainer
	//
	if idConfig.InjectionMode == GCloudMode || idConfig.InjectionMode == UndefinedMode {
		pod.Spec.InitContainers = prependOrReplaceContainer(pod.Spec.InitContainers, gcloudSetupContainer(
			*idConfig.WorkloadIdentityProvider, *idConfig.ServiceAccountEmail, project, m.GcloudImage, idConfig.RunAsUser, m.SetupContainerResources,
		))
	}

	//
	// mutate InitContainers/Containers
	//
	skipContainerNames := map[string]struct{}{
		GCloudSetupInitContainerName: {},
	}
	for _, name := range strings.Split(pod.Annotations[filepath.Join(m.AnnotationDomain, SkipContainersAnnotation)], ",") {
		skipContainerNames[strings.TrimSpace(name)] = struct{}{}
	}
	for i := range pod.Spec.InitContainers {
		ctr := pod.Spec.InitContainers[i]
		if _, ok := skipContainerNames[ctr.Name]; ok {
			continue
		}
		m.mutateContainer(&ctr, volumeMountsToAddOrReplace(idConfig.InjectionMode), envVarsToAddOrReplace(idConfig.InjectionMode), envVarsToAddIfNotPresent(m.DefaultGCloudRegion, project))
		pod.Spec.InitContainers[i] = ctr
	}
	for i := range pod.Spec.Containers {
		ctr := pod.Spec.Containers[i]
		if _, ok := skipContainerNames[ctr.Name]; ok {
			continue
		}
		m.mutateContainer(&ctr, volumeMountsToAddOrReplace(idConfig.InjectionMode), envVarsToAddOrReplace(idConfig.InjectionMode), envVarsToAddIfNotPresent(m.DefaultGCloudRegion, project))
		pod.Spec.Containers[i] = ctr
	}

	return nil
}

func buildExternalCredentialsJson(wiProvider, gsaEmail string) (string, error) {
	aud := fmt.Sprintf("//iam.googleapis.com/%s", wiProvider)
	creds := NewExternalAccountCredentials(aud, gsaEmail)
	credJson, err := creds.Render(false)
	if err != nil {
		return "", err
	}
	return credJson, nil
}

func (m *GCPWorkloadIdentityMutator) mutateContainer(
	ctr *corev1.Container,
	volumeMountsToAdd []corev1.VolumeMount,
	envVarsToAddOrReplace []corev1.EnvVar,
	envVarsToAddIfNotPresent []corev1.EnvVar,
) {
	for i := range volumeMountsToAdd {
		ctr.VolumeMounts = addOrReplaceVolumeMount(ctr.VolumeMounts, volumeMountsToAdd[i])
	}
	for i := range envVarsToAddOrReplace {
		ctr.Env = addOrReplaceEnvVar(ctr.Env, envVarsToAddOrReplace[i])
	}
	for i := range envVarsToAddIfNotPresent {
		ctr.Env = addIfNotPresentEnvVar(ctr.Env, envVarsToAddIfNotPresent[i])
	}
}

func prependOrReplaceContainer(ctrs []corev1.Container, ctr corev1.Container) []corev1.Container {
	replaced := false
	for i, c := range ctrs {
		if c.Name == ctr.Name {
			ctrs[i] = ctr
			replaced = true
			break
		}
	}
	if !replaced {
		ctrs = append([]corev1.Container{ctr}, ctrs...)
	}

	return ctrs
}

func addOrReplaceVolume(volumes []corev1.Volume, volume corev1.Volume) []corev1.Volume {
	replaced := false
	for i, v := range volumes {
		if v.Name == volume.Name {
			volumes[i] = volume
			replaced = true
			break
		}
	}
	if !replaced {
		volumes = append(volumes, volume)
	}
	return volumes
}

func addOrReplaceVolumeMount(volumeMounts []corev1.VolumeMount, volumeMount corev1.VolumeMount) []corev1.VolumeMount {
	replaced := false
	for i, v := range volumeMounts {
		if v.Name == volumeMount.Name {
			volumeMounts[i] = volumeMount
			replaced = true
			break
		}
	}
	if !replaced {
		volumeMounts = append(volumeMounts, volumeMount)
	}
	return volumeMounts
}
func addOrReplaceEnvVar(envVars []corev1.EnvVar, envVar corev1.EnvVar) []corev1.EnvVar {
	replaced := false
	for i, v := range envVars {
		if v.Name == envVar.Name {
			envVars[i] = envVar
			replaced = true
			break
		}
	}
	if !replaced {
		envVars = append(envVars, envVar)
	}
	return envVars
}

func addIfNotPresentEnvVar(envVars []corev1.EnvVar, envVar corev1.EnvVar) []corev1.EnvVar {
	exists := false
	for _, v := range envVars {
		if v.Name == envVar.Name {
			exists = true
			break
		}
	}
	if !exists {
		envVars = append(envVars, envVar)
	}
	return envVars
}
