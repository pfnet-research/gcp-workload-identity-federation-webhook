package webhooks

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GCPWorkloadIdentityMutator", func() {
	namespace := "default"

	workloadProvider := `projects/{PROJECT_NUMBER}/locations/{LOCATION}/workloadIdentityPools/{POOL_ID}/providers/{PROVIDER_ID}`
	project := `project`
	saEmail := fmt.Sprintf("sa@%s.iam.gserviceaccount.com", project)
	audience := `test-audience`
	tokenExpiration := int64(3600)
	var runAsUser int64 = 1000

	var sa corev1.ServiceAccount
	var saDirect corev1.ServiceAccount

	BeforeEach(func() {
		sa = corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "default",
				Annotations: map[string]string{
					idProviderAnnotation:      workloadProvider,
					saEmailAnnotation:         saEmail,
					audienceAnnotation:        audience,
					tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
					runAsUserAnnotation:       fmt.Sprint(runAsUser),
				},
			},
		}
		Expect(k8sClient.Create(ctx, &sa)).NotTo(HaveOccurred())
		// Direct Inject Service Account
		saDirect = corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "direct",
				Annotations: map[string]string{
					idProviderAnnotation:      workloadProvider,
					saEmailAnnotation:         saEmail,
					audienceAnnotation:        audience,
					tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
					runAsUserAnnotation:       fmt.Sprint(runAsUser),
					injectionModeAnnotation:   string(DirectMode),
				},
			},
		}
		Expect(k8sClient.Create(ctx, &saDirect)).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &sa)).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, &saDirect)).NotTo(HaveOccurred())
		Expect(k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(namespace)))
	})

	Describe("Simple Success Case", func() {
		It("should inject gcloud configurations", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-pod",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					InitContainers: []corev1.Container{{
						Name:  "ictr",
						Image: "busybox:test",
					}},
					Containers: []corev1.Container{{
						Name:  "ctr",
						Image: "busybox:test",
					}},
				},
			}

			Expect(k8sClient.Create(ctx, pod)).NotTo(HaveOccurred())
			m := GCPWorkloadIdentityMutator{AnnotationDomain: AnnotationDomainDefault}
			expected := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						idProviderAnnotation:      workloadProvider,
						saEmailAnnotation:         saEmail,
						audienceAnnotation:        audience,
						tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "default",
					InitContainers: []corev1.Container{
						decorateDefault(gcloudSetupContainer(
							workloadProvider,
							saEmail,
							project,
							GcloudImageDefault,
							&runAsUser,
							setupContainerResources,
						)), decorateDefault(corev1.Container{
							Name:         "ictr",
							Image:        "busybox:test",
							VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
							Env:          append(envVarsToAddOrReplace, envVarsToAddIfNotPresent(DefaultGCloudRegionDefault, project)...),
						}),
					},
					Containers: []corev1.Container{decorateDefault(corev1.Container{
						Name:         "ctr",
						Image:        "busybox:test",
						VolumeMounts: volumeMountsToAddOrReplace(GCloudMode),
						Env:          append(envVarsToAddOrReplace, envVarsToAddIfNotPresent(DefaultGCloudRegionDefault, project)...),
					})},
					Volumes: m.volumesToAddOrReplace(audience, tokenExpiration, VolumeModeDefault, GCloudMode),
				},
			}

			Expect(pod.Annotations).To(BeEquivalentTo(expected.Annotations))
			Expect(pod.Spec.ServiceAccountName).To(BeEquivalentTo(expected.Spec.ServiceAccountName))
			Expect(pod.Spec.Volumes).To(BeEquivalentTo(expected.Spec.Volumes))
			Expect(pod.Spec.InitContainers).To(BeEquivalentTo(expected.Spec.InitContainers))
			Expect(pod.Spec.Containers).To(BeEquivalentTo(expected.Spec.Containers))
		})
	})
	Describe("No mutation cases", func() {
		When("Spec.ServiceAccount is empty", func() {
			It("should mutate nothing", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "test-pod",
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: "",
						InitContainers: []corev1.Container{{
							Name:  "ictr",
							Image: "busybox:test",
						}},
						Containers: []corev1.Container{{
							Name:  "ctr",
							Image: "busybox:test",
						}},
					},
				}
				expected := &corev1.Pod{
					ObjectMeta: pod.ObjectMeta,
					Spec: corev1.PodSpec{
						ServiceAccountName: "",
						InitContainers: []corev1.Container{
							decorateDefault(pod.Spec.InitContainers[0]),
						},
						Containers: []corev1.Container{
							decorateDefault(pod.Spec.Containers[0]),
						},
					},
				}

				Expect(k8sClient.Create(ctx, pod)).NotTo(HaveOccurred())
				Expect(pod.Spec.ServiceAccountName).To(BeEquivalentTo(expected.Spec.ServiceAccountName))
				Expect(pod.Spec.Volumes).To(BeEquivalentTo(expected.Spec.Volumes))
				Expect(pod.Spec.InitContainers).To(BeEquivalentTo(expected.Spec.InitContainers))
				Expect(pod.Spec.Containers).To(BeEquivalentTo(expected.Spec.Containers))
			})
		})
		When("Spec.ServiceAccount specifies non-existing ServiceAccount", func() {
			It("should mutate nothing", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "test-pod",
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: "not-found",
						InitContainers: []corev1.Container{{
							Name:  "ictr",
							Image: "busybox:test",
						}},
						Containers: []corev1.Container{{
							Name:  "ctr",
							Image: "busybox:test",
						}},
					},
				}
				expected := &corev1.Pod{
					ObjectMeta: pod.ObjectMeta,
					Spec: corev1.PodSpec{
						ServiceAccountName: "not-found",
						InitContainers: []corev1.Container{
							decorateDefault(pod.Spec.InitContainers[0]),
						},
						Containers: []corev1.Container{
							decorateDefault(pod.Spec.Containers[0]),
						},
					},
				}

				Expect(k8sClient.Create(ctx, pod)).NotTo(HaveOccurred())
				Expect(pod.Spec.ServiceAccountName).To(BeEquivalentTo(expected.Spec.ServiceAccountName))
				Expect(pod.Spec.Volumes).To(BeEquivalentTo(expected.Spec.Volumes))
				Expect(pod.Spec.InitContainers).To(BeEquivalentTo(expected.Spec.InitContainers))
				Expect(pod.Spec.Containers).To(BeEquivalentTo(expected.Spec.Containers))
			})
		})
	})
	Describe("Direct Injection Case", func() {
		It("should inject external cred configurations via annotation & downwardAPI Volume", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-pod",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "direct",
					InitContainers: []corev1.Container{{
						Name:  "ictr",
						Image: "busybox:test",
					}},
					Containers: []corev1.Container{{
						Name:  "ctr",
						Image: "busybox:test",
					}},
				},
			}

			Expect(k8sClient.Create(ctx, pod)).NotTo(HaveOccurred())
			m := GCPWorkloadIdentityMutator{AnnotationDomain: AnnotationDomainDefault}
			expected := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						idProviderAnnotation:      workloadProvider,
						saEmailAnnotation:         saEmail,
						audienceAnnotation:        audience,
						tokenExpirationAnnotation: fmt.Sprint(tokenExpiration),
						externalConfigAnnotation:  buildExternalCredentialsJson(workloadProvider, saEmail),
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "direct",
					InitContainers: []corev1.Container{
						decorateDefault(corev1.Container{
							Name:         "ictr",
							Image:        "busybox:test",
							VolumeMounts: volumeMountsToAddOrReplace(DirectMode),
							Env:          append(envVarsToAddOrReplace, envVarsToAddIfNotPresent(DefaultGCloudRegionDefault, project)...),
						}),
					},
					Containers: []corev1.Container{decorateDefault(corev1.Container{
						Name:         "ctr",
						Image:        "busybox:test",
						VolumeMounts: volumeMountsToAddOrReplace(DirectMode),
						Env:          append(envVarsToAddOrReplace, envVarsToAddIfNotPresent(DefaultGCloudRegionDefault, project)...),
					})},
					Volumes: m.volumesToAddOrReplace(audience, tokenExpiration, VolumeModeDefault, DirectMode),
				},
			}

			Expect(pod.Annotations).To(BeEquivalentTo(expected.Annotations))
			Expect(pod.Spec.ServiceAccountName).To(BeEquivalentTo(expected.Spec.ServiceAccountName))
			Expect(pod.Spec.Volumes).To(BeEquivalentTo(expected.Spec.Volumes))
			Expect(pod.Spec.InitContainers).To(BeEquivalentTo(expected.Spec.InitContainers))
			Expect(pod.Spec.Containers).To(BeEquivalentTo(expected.Spec.Containers))
		})
	})
})

func decorateDefault(ctr corev1.Container) corev1.Container {
	ctr.TerminationMessagePath = "/dev/termination-log"
	ctr.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	ctr.ImagePullPolicy = corev1.PullIfNotPresent
	return ctr
}
