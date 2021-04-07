package klusterlet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	"github.com/mdelder/failover/pkg/helpers"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/pflag"
)

// FailoverAgentOptions holds configuration for spoke cluster agent
type FailoverAgentOptions struct {
	ComponentNamespace       string
	ClusterName              string
	AgentName                string
	BootstrapKubeconfig      string
	HubKubeconfigSecret      string
	HubKubeconfigDir         string
	SpokeExternalServerURLs  []string
	ClusterHealthCheckPeriod time.Duration
	MaxCustomClusterClaims   int
}

// NewFailoverAgentOptions returns a FailoverAgentOptions
func NewFailoverAgentOptions() *FailoverAgentOptions {
	return &FailoverAgentOptions{
		HubKubeconfigSecret:      "hub-kubeconfig-secret",
		HubKubeconfigDir:         "/spoke/hub-kubeconfig",
		ClusterHealthCheckPeriod: 1 * time.Minute,
		MaxCustomClusterClaims:   20,
	}
}

// RunSpokeAgent starts the controllers on spoke agent to register to the hub.
//
// The spoke agent uses three kubeconfigs for different concerns:
// - The 'spoke' kubeconfig: used to communicate with the spoke cluster where
//   the agent is running.
// - The 'bootstrap' kubeconfig: used to communicate with the hub in order to
//   submit a CertificateSigningRequest, begin the join flow with the hub, and
//   to write the 'hub' kubeconfig.
// - The 'hub' kubeconfig: used to communicate with the hub using a signed
//   certificate from the hub.
//
// RunSpokeAgent handles the following scenarios:
//   #1. Bootstrap kubeconfig is valid and there is no valid hub kubeconfig in secret
//   #2. Both bootstrap kubeconfig and hub kubeconfig are valid
//   #3. Bootstrap kubeconfig is invalid (e.g. certificate expired) and hub kubeconfig is valid
//   #4. Neither bootstrap kubeconfig nor hub kubeconfig is valid
//
// A temporary ClientCertForHubController with bootstrap kubeconfig is created
// and started if the hub kubeconfig does not exist or is invalid and used to
// create a valid hub kubeconfig. Once the hub kubeconfig is valid, the
// temporary controller is stopped and the main controllers are started.
func (o *FailoverAgentOptions) RunFailoverAgent(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {

	klog.Infof("Cluster name is %q and agent name is %q", o.ClusterName, o.AgentName)

	klog.Infof("Let's get to work!")

	// // create kube client
	// spokeKubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	// if err != nil {
	// 	return err
	// }

	// if err := o.Complete(spokeKubeClient.CoreV1(), ctx, controllerContext.EventRecorder); err != nil {
	// 	klog.Fatal(err)
	// }

	// if err := o.Validate(); err != nil {
	// 	klog.Fatal(err)
	// }

	// klog.Infof("Cluster name is %q and agent name is %q", o.ClusterName, o.AgentName)

	// // create shared informer factory for spoke cluster
	// spokeKubeInformerFactory := informers.NewSharedInformerFactory(spokeKubeClient, 10*time.Minute)
	// namespacedSpokeKubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(spokeKubeClient, 10*time.Minute, informers.WithNamespace(o.ComponentNamespace))

	// // get spoke cluster CA bundle
	// spokeClusterCABundle, err := o.getSpokeClusterCABundle(controllerContext.KubeConfig)
	// if err != nil {
	// 	return err
	// }

	// // load bootstrap client config and create bootstrap clients
	// bootstrapClientConfig, err := clientcmd.BuildConfigFromFlags("", o.BootstrapKubeconfig)
	// if err != nil {
	// 	return fmt.Errorf("unable to load bootstrap kubeconfig from file %q: %w", o.BootstrapKubeconfig, err)
	// }
	// bootstrapKubeClient, err := kubernetes.NewForConfig(bootstrapClientConfig)
	// if err != nil {
	// 	return err
	// }
	// bootstrapClusterClient, err := clusterv1client.NewForConfig(bootstrapClientConfig)
	// if err != nil {
	// 	return err
	// }

	// // start a SpokeClusterCreatingController to make sure there is a spoke cluster on hub cluster
	// spokeClusterCreatingController := managedcluster.NewManagedClusterCreatingController(
	// 	o.ClusterName, o.SpokeExternalServerURLs,
	// 	spokeClusterCABundle,
	// 	bootstrapClusterClient,
	// 	controllerContext.EventRecorder,
	// )
	// go spokeClusterCreatingController.Run(ctx, 1)

	// hubKubeconfigSecretController := hubclientcert.NewHubKubeconfigSecretController(
	// 	o.HubKubeconfigDir, o.ComponentNamespace, o.HubKubeconfigSecret,
	// 	spokeKubeClient.CoreV1(),
	// 	namespacedSpokeKubeInformerFactory.Core().V1().Secrets(),
	// 	controllerContext.EventRecorder,
	// )
	// go hubKubeconfigSecretController.Run(ctx, 1)

	// //

	// controllerContext.EventRecorder.Event("HubClientConfigReady", "Client config for hub is ready.")

	// <-ctx.Done()
	return nil
}

// AddFlags registers flags for Agent
func (o *FailoverAgentOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.ClusterName, "cluster-name", o.ClusterName,
		"If non-empty, will use as cluster name instead of generated random name.")
	fs.StringVar(&o.BootstrapKubeconfig, "bootstrap-kubeconfig", o.BootstrapKubeconfig,
		"The path of the kubeconfig file for agent bootstrap.")
	fs.StringVar(&o.HubKubeconfigSecret, "hub-kubeconfig-secret", o.HubKubeconfigSecret,
		"The name of secret in component namespace storing kubeconfig for hub.")
	fs.StringVar(&o.HubKubeconfigDir, "hub-kubeconfig-dir", o.HubKubeconfigDir,
		"The mount path of hub-kubeconfig-secret in the container.")
	fs.StringArrayVar(&o.SpokeExternalServerURLs, "spoke-external-server-urls", o.SpokeExternalServerURLs,
		"A list of reachable spoke cluster api server URLs for hub cluster.")
	fs.DurationVar(&o.ClusterHealthCheckPeriod, "cluster-healthcheck-period", o.ClusterHealthCheckPeriod,
		"The period to check managed cluster kube-apiserver health")
	fs.IntVar(&o.MaxCustomClusterClaims, "max-custom-cluster-claims", o.MaxCustomClusterClaims,
		"The max number of custom cluster claims to expose.")
}

// Validate verifies the inputs.
func (o *FailoverAgentOptions) Validate() error {
	if o.BootstrapKubeconfig == "" {
		return errors.New("bootstrap-kubeconfig is required")
	}

	if o.ClusterName == "" {
		return errors.New("cluster name is empty")
	}

	if o.AgentName == "" {
		return errors.New("agent name is empty")
	}

	// if SpokeExternalServerURLs is specified we validate every URL in it, we expect the spoke external server URL is https
	if len(o.SpokeExternalServerURLs) != 0 {
		for _, serverURL := range o.SpokeExternalServerURLs {
			if !helpers.IsValidHTTPSURL(serverURL) {
				return errors.New(fmt.Sprintf("%q is invalid", serverURL))
			}
		}
	}

	if o.ClusterHealthCheckPeriod <= 0 {
		return errors.New("cluster healthcheck period must greater than zero")
	}

	return nil
}
