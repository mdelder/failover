package klusterlet

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/mdelder/failover/pkg/klusterlet"
	"github.com/mdelder/failover/pkg/version"
)

func NewFailoverAgent() *cobra.Command {
	agentOptions := klusterlet.NewFailoverAgentOptions()
	cmd := controllercmd.
		NewControllerCommandConfig("failover-agent", version.Get(), agentOptions.RunFailoverAgent).
		NewCommand()
	cmd.Use = "agent"
	cmd.Short = "Start the Cluster Failover Agent"

	agentOptions.AddFlags(cmd.Flags())
	return cmd
}
