package webhooks

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	_ inject.Logger             = &GCPWorkloadIdentityMutator{}
	_ inject.Client             = &GCPWorkloadIdentityMutator{}
	_ admission.Handler         = &GCPWorkloadIdentityMutator{}
	_ admission.DecoderInjector = &GCPWorkloadIdentityMutator{}
)

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=ignore,groups="",resources=pods,verbs=create,versions=v1,name=mpod.kb.io,admissionReviewVersions=v1,sideEffects=None

// GCPWorkloadIdentityMutator inject configurations for containers to acquire workload federated identity automatically
type GCPWorkloadIdentityMutator struct {
	AnnotationDomain        string
	DefaultAudience         string
	DefaultTokenExpiration  time.Duration
	MinTokenExpration       time.Duration
	DefaultGCloudRegion     string
	GcloudImage             string
	SetupContainerResources *corev1.ResourceRequirements

	logger  logr.Logger
	decoder *admission.Decoder
	client.Client
}

//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch

// Handle implements admission.Handler
func (m *GCPWorkloadIdentityMutator) Handle(ctx context.Context, ar admission.Request) admission.Response {
	var err error
	pod := &corev1.Pod{}

	err = m.decoder.Decode(ar, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	logger := m.logger.WithValues("Pod", pod.Namespace+"/"+pod.Name)

	if pod.Spec.ServiceAccountName == "" {
		logger.V(2).Info("Skip processing because Spec.ServiceAccountName is empty (this might be a mirror pod)")
		return admission.Allowed("Skipped processing because Spec.ServiceAccountName is empty (this might be a mirror pod)")
	}

	sa := corev1.ServiceAccount{}
	err = m.Get(ctx, types.NamespacedName{Namespace: ar.Namespace, Name: pod.Spec.ServiceAccountName}, &sa)
	if err != nil && apierrors.IsNotFound(err) {
		logger.V(2).Info("Skip processing because ServiceAccount is not found", "ServiceAccount", pod.Spec.ServiceAccountName)
		return admission.Allowed("Skip processing because ServiceAccount is not found")
	}
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	idConfig, err := NewGCPWorkloadIdentityConfig(m.AnnotationDomain, sa)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if idConfig == nil {
		return admission.Allowed("")
	}

	if err := m.mutatePod(pod, *idConfig); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(ar.Object.Raw, marshaledPod)
}

func (m *GCPWorkloadIdentityMutator) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("setup-gcp-wrokload-identity-mutator")

	saInformer, err := mgr.GetCache().GetInformer(ctx, &corev1.ServiceAccount{})
	if err != nil {
		logger.Error(err, "Failed to get ServiceAccount informer")
		return err
	}
	saInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	mgr.GetWebhookServer().Register("/mutate-v1-pod", &webhook.Admission{Handler: m})
	return nil
}

// InjectClient implements inject.Client
func (m *GCPWorkloadIdentityMutator) InjectClient(c client.Client) error {
	m.Client = c
	return nil
}

// InjectLogger implements inject.Logger
func (m *GCPWorkloadIdentityMutator) InjectLogger(l logr.Logger) error {
	m.logger = l.WithName("gcp-wrokload-identity-mutator")
	return nil
}

// InjectDecoder implements admission.DecoderInjector
func (m *GCPWorkloadIdentityMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
