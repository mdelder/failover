package klusterlet

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/mdelder/failover/pkg/helpers"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/spf13/pflag"

	"github.com/open-cluster-management/registration/pkg/spoke/hubclientcert"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/operator/events"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

const (
	// agentNameLength is the length of the spoke agent name which is generated automatically
	agentNameLength = 5
	// defaultSpokeComponentNamespace is the default namespace in which the spoke agent is deployed
	defaultSpokeComponentNamespace = "open-cluster-management"
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

	klog.Infof("Let's get to work!")

	// // create kube client
	agentKubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	if err := o.Complete(agentKubeClient.CoreV1(), ctx, controllerContext.EventRecorder); err != nil {
		klog.Fatal(err)
	}

	if err := o.Validate(); err != nil {
		klog.Fatal(err)
	}

	klog.Infof("Cluster name is %q and agent name is %q", o.ClusterName, o.AgentName)

	// // create shared informer factory for spoke cluster
	// agentKubeInformerFactory := informers.NewSharedInformerFactory(agentKubeClient, 10*time.Minute)
	namespacedAgentKubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(agentKubeClient, 10*time.Minute, informers.WithNamespace(o.ComponentNamespace))

	hubKubeconfigSecretController := hubclientcert.NewHubKubeconfigSecretController(
		o.HubKubeconfigDir, o.ComponentNamespace, o.HubKubeconfigSecret,
		agentKubeClient.CoreV1(),
		namespacedAgentKubeInformerFactory.Core().V1().Secrets(),
		controllerContext.EventRecorder,
	)
	go hubKubeconfigSecretController.Run(ctx, 1)

	<-ctx.Done()
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

// Complete fills in missing values.
func (o *FailoverAgentOptions) Complete(coreV1Client corev1client.CoreV1Interface, ctx context.Context, recorder events.Recorder) error {
	// get component namespace of spoke agent
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		o.ComponentNamespace = defaultSpokeComponentNamespace
	} else {
		o.ComponentNamespace = string(nsBytes)
	}

	// // dump data in hub kubeconfig secret into file system if it exists
	// err = hubclientcert.DumpSecret(coreV1Client, o.ComponentNamespace, o.HubKubeconfigSecret,
	// 	o.HubKubeconfigDir, ctx, recorder)
	// if err != nil {
	// 	return err
	// }

	// load or generate cluster/agent names
	o.ClusterName, o.AgentName = o.getOrGenerateClusterAgentNames()

	return nil
}

// getOrGenerateClusterAgentNames returns cluster name and agent name.
// Rules for picking up cluster name:
//   1. Use cluster name from input arguments if 'cluster-name' is specified;
//   2. Parse cluster name from the common name of the certification subject if the certification exists;
//   3. Fallback to cluster name in the mounted secret if it exists;
//   4. TODO: Read cluster name from openshift struct if the agent is running in an openshift cluster;
//   5. Generate a random cluster name then;

// Rules for picking up agent name:
//   1. Parse agent name from the common name of the certification subject if the certification exists;
//   2. Fallback to agent name in the mounted secret if it exists;
//   3. Generate a random agent name then;
func (o *FailoverAgentOptions) getOrGenerateClusterAgentNames() (string, string) {
	clusterName := generateClusterName()
	// generate random agent name
	agentName := generateAgentName()

	return clusterName, agentName
}

// generateClusterName generates a name for spoke cluster
func generateClusterName() string {
	return string(uuid.NewUUID())
}

// generateAgentName generates a random name for spoke cluster agent
func generateAgentName() string {
	return utilrand.String(agentNameLength)
}
