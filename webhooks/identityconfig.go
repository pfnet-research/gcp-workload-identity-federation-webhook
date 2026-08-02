package webhooks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	workloadIdentityProviderRegex = regexp.MustCompile(`^projects/.+/locations/.+/workloadIdentityPools/.+/providers/.+$`)
	workloadIdentityProviderFmt   = `projects/{ProjectNumber}/locations/{Location}/workloadIdentityPools/{PoolId}/providers/{ProviderId}`
)

type GCPWorkloadIdentityConfig struct {
	WorkloadIdentityProvider *string
	ServiceAccountEmail      *string
	RunAsUser                *int64
	InjectionMode            InjectionMode
	TokenExchangeMode        TokenExchangeMode
	ProjectID                *string

	Audience               *string
	TokenExpirationSeconds *int64
}

type InjectionMode string

const (
	UndefinedMode InjectionMode = ""
	GCloudMode    InjectionMode = "gcloud"
	DirectMode    InjectionMode = "direct"
)

type TokenExchangeMode string

const (
	ServiceAccountMode TokenExchangeMode = "service-account"
	DirectAccessMode   TokenExchangeMode = "direct-access"
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
		cfg.ServiceAccountEmail = &v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, AudienceAnnotation)]; ok {
		cfg.Audience = &v
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, ProjectIDAnnotation)]; ok {
		cfg.ProjectID = &v
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
			return nil, fmt.Errorf("%s mode must be '%s', '%s' or unset", filepath.Join(annotationDomain, InjectionModeAnnotation), DirectMode, GCloudMode)
		}
	} else {
		cfg.InjectionMode = UndefinedMode
	}

	if v, ok := sa.Annotations[filepath.Join(annotationDomain, TokenExchangeModeAnnotation)]; ok {
		switch TokenExchangeMode(strings.ToLower(v)) {
		case ServiceAccountMode:
			cfg.TokenExchangeMode = ServiceAccountMode
		case DirectAccessMode:
			cfg.TokenExchangeMode = DirectAccessMode
		default:
			return nil, fmt.Errorf("%s mode must be '%s', '%s' or unset", filepath.Join(annotationDomain, TokenExchangeModeAnnotation), ServiceAccountMode, DirectAccessMode)
		}
	} else {
		cfg.TokenExchangeMode = ServiceAccountMode
	}

	switch cfg.TokenExchangeMode {
	case DirectAccessMode:
		if cfg.WorkloadIdentityProvider == nil {
			return nil, fmt.Errorf("%s is required when %s is '%s'",
				filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation),
				filepath.Join(annotationDomain, TokenExchangeModeAnnotation),
				DirectAccessMode,
			)
		}
		if cfg.ServiceAccountEmail != nil {
			ctrllog.Log.WithName("identityconfig").Info(
				"ignoring service-account-email annotation in direct-access token-exchange-mode",
				"serviceaccount", sa.Namespace+"/"+sa.Name,
			)
			cfg.ServiceAccountEmail = nil
		}
	default: // ServiceAccountMode
		if cfg.WorkloadIdentityProvider == nil && cfg.ServiceAccountEmail == nil {
			return nil, nil
		}
		if cfg.WorkloadIdentityProvider == nil || cfg.ServiceAccountEmail == nil {
			return nil, fmt.Errorf("%s, %s must set at a time", filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation), filepath.Join(annotationDomain, ServiceAccountEmailAnnotation))
		}
	}

	// Invariant: WorkloadIdentityProvider is non-nil here — both token-exchange-mode branches above ensure it.
	if !workloadIdentityProviderRegex.Match([]byte(*cfg.WorkloadIdentityProvider)) {
		return nil, fmt.Errorf("%s must be form of %s", filepath.Join(annotationDomain, WorkloadIdentityProviderAnnotation), workloadIdentityProviderFmt)
	}

	return cfg, nil
}
