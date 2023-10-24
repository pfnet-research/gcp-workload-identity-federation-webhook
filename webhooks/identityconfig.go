package webhooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

var (
	workloadIdentityProviderRegex = regexp.MustCompile(`^projects/.+/locations/.+/workloadIdentityPools/.+/providers/.+$`)
	workloadIdentityProviderFmt   = `projects/{ProjectNumber}/locations/{Location}/workloadIdentityPools/{PoolId}/providers/{ProviderId}`
)

type GCPWorkloadIdentityConfig struct {
	WorkloadIdentityProvider *string
	ServiceAccountEmail      *string
	RunAsUser                *int64

	Audience               *string
	TokenExpirationSeconds *int64
}

func NewGCPWorkloadIdentityConfig(
	annotationDomain string,
	sa corev1.ServiceAccount,
) (*GCPWorkloadIdentityConfig, error) {
	cfg := &GCPWorkloadIdentityConfig{}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation)]; ok {
		cfg.WorkloadIdentityProvider = &v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, ServiceAccountEmailAnnotation)]; ok {
		cfg.ServiceAccountEmail = &v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, AudienceAnnotation)]; ok {
		cfg.Audience = &v
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

	if cfg.WorkloadIdentityProvider == nil && cfg.ServiceAccountEmail == nil {
		return nil, nil
	}

	if cfg.WorkloadIdentityProvider == nil || cfg.ServiceAccountEmail == nil {
		return nil, fmt.Errorf("%s, %s must set at a time", filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation), filepath.Join(annotationDomain, TokenExpirationAnnotation))
	}

	if !workloadIdentityProviderRegex.Match([]byte(*cfg.WorkloadIdentityProvider)) {
		return nil, fmt.Errorf("%s must be form of %s", filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation), workloadIdentityProviderFmt)
	}

	return cfg, nil
}
