/*
Copyright 2022 Preferred Networks, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.pfidev.jp/kubernetes/gcp-workload-identity-federation-webhook/webhooks"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	ctx := ctrl.SetupSignalHandler()

	metricsAddr := flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	probeAddr := flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	annotationPrefix := flag.String("annotation-prefix", webhooks.AnnotationDomainDefault, "The Service Account annotation to look for")
	defaultAudience := flag.String("token-audience", webhooks.AudienceDefault, "The default audience for tokens. Can be overridden by annotation")
	defaultTokenExpiration := flag.Duration("token-expiration", webhooks.DefaultTokenExpirationDefault, "The token expiration")
	defaultRegion := flag.String("gcp-default-region", "", "If set, CLOUDSDK_COMPUTE_REGION will be set to this value in mutated containers")
	gCloudImage := flag.String("gcloud-image", webhooks.GcloudImageDefault, "Container image for the init container setting up GCloud SDK")
	tokenDefaultMode := flag.Int("token-default-mode", webhooks.VolumeModeDefault, "DefaultMode for the token volume. CAUTION: if you allow reading from others (e.g. '0444'), the token can read from anyone who can log in to the node.")
	setupContainerResources := flag.String("setup-container-resources", webhooks.SetupContainerResources, `Resource spec in json for the init container setting up GCloud SDK, e.g. '{"requests":{"cpu":"100m"}}'`)

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	var setupContainerResourceRequirements *corev1.ResourceRequirements
	if *setupContainerResources != "" {
		setupContainerResourceRequirements = &corev1.ResourceRequirements{}
		if err := json.Unmarshal([]byte(*setupContainerResources), setupContainerResourceRequirements); err != nil {
			setupLog.Error(err, "unable to parse the value of --setup-container-resources")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: *metricsAddr,
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port: 944,
			},
		),
		HealthProbeBindAddress: *probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&webhooks.GCPWorkloadIdentityMutator{
		AnnotationDomain:        *annotationPrefix,
		DefaultAudience:         *defaultAudience,
		DefaultTokenExpiration:  *defaultTokenExpiration,
		MinTokenExpration:       webhooks.MinTokenExprationDefault,
		DefaultGCloudRegion:     *defaultRegion,
		GcloudImage:             *gCloudImage,
		DefaultMode:             int32(*tokenDefaultMode),
		SetupContainerResources: setupContainerResourceRequirements,
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to setup gcp-workload-identity-mutator")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
