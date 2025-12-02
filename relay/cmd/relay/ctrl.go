package relay

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	applogger "github.com/wafieio/wafie/logger"
	"github.com/wafieio/wafie/relay/pkg/control"
	"go.uber.org/zap"
	discoveryv1 "k8s.io/api/discovery/v1"
)

func init() {
	controllerCmd.PersistentFlags().StringP("api-addr", "a", "http://localhost:8080", "API address")
	controllerCmd.PersistentFlags().StringP("node-name", "n", "", "K8s node name")
	viper.BindPFlag("api-addr", controllerCmd.PersistentFlags().Lookup("api-addr"))
	viper.BindPFlag("node-name", controllerCmd.PersistentFlags().Lookup("node-name"))
	startCmd.AddCommand(controllerCmd)
}

var controllerCmd = &cobra.Command{
	Use:   "relay-instance-controller",
	Short: "start relay instance controller",
	Run: func(cmd *cobra.Command, args []string) {
		zap.S().Info("starting relay instance controller")
		epsCh := make(chan *discoveryv1.EndpointSlice, 100)
		// start relay controller
		relayCtrl, err := control.NewController(
			viper.GetString("api-addr"),
			viper.GetString("node-name"),
			epsCh,
			applogger.NewLogger(),
		)
		if err != nil {
			panic(err)
		}
		relayCtrl.Run()
		controllerShutdown()
	},
}

func controllerShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	gracefullyExit := func(sig os.Signal) {
		zap.S().Infof("shutting down with sig: %s, bye bye 👋\n", sig.String())
		if s, ok := sig.(syscall.Signal); ok {
			os.Exit(128 + int(s))
		}
		os.Exit(1)
	}
	for {
		select {
		case sig := <-sigCh:
			gracefullyExit(sig)
		}
	}
}
