package webhooks

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NewGCPWorkloadIdentityConfig", func() {
	workloadProvider := `projects/{PROJECT_NUMBER}/locations/{LOCATION}/workloadIdentityPools/{POOL_ID}/providers/{PROVIDER_ID}`
	saEmail := `sa@project.iam.gserviceaccount.com`
	audience := `test-audience`
	tokenExpiration := int64(3600)

	Describe("Success Case", func() {
		When("ServiceAccount has no annotation", func() {
			It("can create GCPWorkloadIdentityConfig", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeNil())
			})
		})
		When("ServiceAccount with minimal required annotations", func() {
			It("can create GCPWorkloadIdentityConfig", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation: workloadProvider,
							saEmailAnnotation:    saEmail,
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					Audience:                 nil,
					TokenExpirationSeconds:   nil,
					TokenExchangeMode:        ServiceAccountMode,
				}))
			})
		})
		When("ServiceAccount with full annotations", func() {
			It("can create GCPWorkloadIdentityConfig", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:      workloadProvider,
							saEmailAnnotation:         saEmail,
							audienceAnnotation:        audience,
							tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					Audience:                 &audience,
					TokenExpirationSeconds:   &tokenExpiration,
					TokenExchangeMode:        ServiceAccountMode,
				}))
			})
		})
		When("ServiceAccount with 'direct' injection mode annotation", func() {
			It("can create GCPWorkloadIdentityConfig", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:      workloadProvider,
							saEmailAnnotation:         saEmail,
							audienceAnnotation:        audience,
							tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
							injectionModeAnnotation:   string(DirectMode),
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					Audience:                 &audience,
					TokenExpirationSeconds:   &tokenExpiration,
					InjectionMode:            DirectMode,
					TokenExchangeMode:        ServiceAccountMode,
				}))
			})
		})
		When("ServiceAccount with 'gcloud' injection mode annotation", func() {
			It("can create GCPWorkloadIdentityConfig", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:      workloadProvider,
							saEmailAnnotation:         saEmail,
							audienceAnnotation:        audience,
							tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
							injectionModeAnnotation:   string(GCloudMode),
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					Audience:                 &audience,
					TokenExpirationSeconds:   &tokenExpiration,
					InjectionMode:            GCloudMode,
					TokenExchangeMode:        ServiceAccountMode,
				}))
			})
		})
		When("ServiceAccount with 'direct-access' token-exchange-mode only (no service-account-email)", func() {
			It("can create GCPWorkloadIdentityConfig", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:        workloadProvider,
							tokenExchangeModeAnnotation: string(DirectAccessMode),
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      nil,
					TokenExchangeMode:        DirectAccessMode,
				}))
			})
		})
		When("ServiceAccount with 'direct-access' mode ignores service-account-email", func() {
			It("returns config with ServiceAccountEmail cleared", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:        workloadProvider,
							saEmailAnnotation:           saEmail,
							tokenExchangeModeAnnotation: string(DirectAccessMode),
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      nil,
					TokenExchangeMode:        DirectAccessMode,
				}))
			})
		})
		When("ServiceAccount with explicit 'service-account' token-exchange-mode", func() {
			It("behaves identically to default", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:        workloadProvider,
							saEmailAnnotation:           saEmail,
							tokenExchangeModeAnnotation: string(ServiceAccountMode),
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					TokenExchangeMode:        ServiceAccountMode,
				}))
			})
		})
		When("ServiceAccount with mixed-case 'DIRECT-ACCESS' token-exchange-mode", func() {
			It("parses case-insensitively", func() {
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:        workloadProvider,
							tokenExchangeModeAnnotation: "DIRECT-ACCESS",
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig.TokenExchangeMode).To(Equal(DirectAccessMode))
			})
		})
		When("ServiceAccount with project-id annotation in direct-access mode", func() {
			It("captures the explicit ProjectID", func() {
				projectID := `my-project`
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:        workloadProvider,
							tokenExchangeModeAnnotation: string(DirectAccessMode),
							projectIDAnnotation:         projectID,
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					TokenExchangeMode:        DirectAccessMode,
					ProjectID:                &projectID,
				}))
			})
		})
		When("ServiceAccount with project-id annotation in service-account mode", func() {
			It("captures the explicit ProjectID alongside the SA email", func() {
				projectID := `override-project`
				sa := corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation: workloadProvider,
							saEmailAnnotation:    saEmail,
							projectIDAnnotation:  projectID,
						},
					},
				}
				idConfig, err := NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(err).NotTo(HaveOccurred())
				Expect(idConfig).To(BeEquivalentTo(&GCPWorkloadIdentityConfig{
					WorkloadIdentityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					TokenExchangeMode:        ServiceAccountMode,
					ProjectID:                &projectID,
				}))
			})
		})
	})
	Describe("Failure Case", func() {
		var sa corev1.ServiceAccount
		var idConfig *GCPWorkloadIdentityConfig
		var err error
		When("ServiceAccount without some required annotations", func() {
			It("should raise error", func() {
				By("without service-account-email annotation")
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation: workloadProvider,
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("must set at a time")))

				By("without workload-identity-provider annotation")
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							saEmailAnnotation: saEmail,
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("must set at a time")))
			})
		})
		When("ServiceAccount with malformed workload-identity-provider annotation", func() {
			It("should raise error", func() {
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation: "malformed-workload-identity-provider",
							saEmailAnnotation:    saEmail,
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("must be form of")))
			})
		})
		When("ServiceAccount with unparsable token-expiration annotation", func() {
			It("should raise error", func() {
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:      workloadProvider,
							saEmailAnnotation:         saEmail,
							tokenExpirationAnnotation: "not integer",
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("must be positive integer string")))
			})
		})
		When("ServiceAccount with unparsable injection mode annotation", func() {
			It("should raise error", func() {
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:    workloadProvider,
							saEmailAnnotation:       saEmail,
							injectionModeAnnotation: "not-valid",
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("mode must be")))
			})
		})
		When("ServiceAccount with unparsable token-exchange-mode annotation", func() {
			It("should raise error", func() {
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							idProviderAnnotation:        workloadProvider,
							saEmailAnnotation:           saEmail,
							tokenExchangeModeAnnotation: "not-valid",
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("mode must be")))
			})
		})
		When("ServiceAccount with 'direct-access' mode but no workload-identity-provider", func() {
			It("should raise error", func() {
				sa = corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							tokenExchangeModeAnnotation: string(DirectAccessMode),
						},
					},
				}
				idConfig, err = NewGCPWorkloadIdentityConfig(annotaitonDomain, sa)
				Expect(idConfig).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("workload-identity-provider")))
			})
		})
	})
})
