package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	annotaitonDomain          = AnnotationDomainDefault
	idProviderAnnotation      = filepath.Join(annotaitonDomain, WorkloadIdentityProviderAnnotation)
	saEmailAnnotation         = filepath.Join(annotaitonDomain, ServiceAccountEmailAnnotation)
	audienceAnnotation        = filepath.Join(annotaitonDomain, AudienceAnnotation)
	tokenExpirationAnnotation = filepath.Join(annotaitonDomain, TokenExpirationAnnotation)
	runAsUserAnnotation       = filepath.Join(annotaitonDomain, RunAsUserAnnotation)
	injectionModeAnnotation   = filepath.Join(annotaitonDomain, InjectionModeAnnotation)
	externalConfigAnnotation  = filepath.Join(annotaitonDomain, ExternalCredentialsJsonAnnotation)
	setupContainerResources   = &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("100m"),
		},
	}

	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestWebhook(t *testing.T) {
	format.MaxLength = 40000
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhooks Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.TODO())

	log.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "config", "webhook")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Host:    testEnv.WebhookInstallOptions.LocalServingHost,
				Port:    testEnv.WebhookInstallOptions.LocalServingPort,
				CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
			},
		),
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&GCPWorkloadIdentityMutator{
		AnnotationDomain:        AnnotationDomainDefault,
		DefaultAudience:         AudienceDefault,
		DefaultTokenExpiration:  DefaultTokenExpirationDefault,
		MinTokenExpration:       MinTokenExprationDefault,
		DefaultGCloudRegion:     DefaultGCloudRegionDefault,
		GcloudImage:             GcloudImageDefault,
		DefaultMode:             VolumeModeDefault,
		SetupContainerResources: setupContainerResources,
	}).SetupWithManager(ctx, k8sManager)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", testEnv.WebhookInstallOptions.LocalServingHost, testEnv.WebhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
