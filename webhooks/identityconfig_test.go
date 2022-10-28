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
					WorkloadIdeneityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					Audience:                 nil,
					TokenExpirationSeconds:   nil,
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
					WorkloadIdeneityProvider: &workloadProvider,
					ServiceAccountEmail:      &saEmail,
					Audience:                 &audience,
					TokenExpirationSeconds:   &tokenExpiration,
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
	})
})
