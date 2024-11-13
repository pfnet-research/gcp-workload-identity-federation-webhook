package webhooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var (
	workloadIdentityProviderRegex = regexp.MustCompile(`^projects/.+/locations/.+/workloadIdentityPools/.+/providers/.+$`)
	workloadIdentityProviderFmt   = `projects/{ProjectNumber}/locations/{Location}/workloadIdentityPools/{PoolId}/providers/{ProviderId}`
)

type GCPWorkloadIdentityConfig struct {
	Project                  *string
	WorkloadIdentityProvider *string
	ServiceAccountEmail      string
	RunAsUser                *int64
	InjectionMode            InjectionMode

	Audience               *string
	TokenExpirationSeconds *int64
}

type InjectionMode string

const (
	UndefinedMode InjectionMode = ""
	GCloudMode    InjectionMode = "gcloud"
	DirectMode    InjectionMode = "direct"
)

func NewGCPWorkloadIdentityConfig(
	annotationDomain string,
	sa corev1.ServiceAccount,
) (*GCPWorkloadIdentityConfig, error) {
	cfg := &GCPWorkloadIdentityConfig{}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation)]; ok {
		cfg.WorkloadIdentityProvider = &v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, ServiceAccountEmailAnnotation)]; ok {
		cfg.ServiceAccountEmail = v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, AudienceAnnotation)]; ok {
		cfg.Audience = &v
	}
	if v, ok := sa.Annotations[filepath.Join(annotationDomain, ProjectAnnotation)]; ok {
		cfg.Project = &v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, TokenExpirationAnnotation)]; ok {
		seconds, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must be positive integer string: %w", filepath.Join(annotationDomain, TokenExpirationAnnotation), err)
		}
		cfg.TokenExpirationSeconds = &seconds
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, RunAsUserAnnotation)]; ok {
		userId, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must be positive integer string: %w", filepath.Join(annotationDomain, RunAsUserAnnotation), err)
		}
		cfg.RunAsUser = &userId
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, InjectionModeAnnotation)]; ok {
		switch InjectionMode(strings.ToLower(v)) {
		case DirectMode:
			cfg.InjectionMode = DirectMode
		case GCloudMode:
			cfg.InjectionMode = GCloudMode
		default:
			return nil, fmt.Errorf("%s mode must be '%s', '%s' or unset.", filepath.Join(annotationDomain, InjectionModeAnnotation), DirectMode, GCloudMode)
		}
	} else {
		cfg.InjectionMode = UndefinedMode
	}

	if cfg.WorkloadIdentityProvider == nil {
		return nil, nil
	}

	if !workloadIdentityProviderRegex.Match([]byte(*cfg.WorkloadIdentityProvider)) {
		return nil, fmt.Errorf("%s must be form of %s", filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation), workloadIdentityProviderFmt)
	}

	return cfg, nil
}
