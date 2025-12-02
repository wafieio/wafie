package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	hsrv "github.com/wafieio/wafie/apisrv/pkg/healthchecksrv"
	"github.com/wafieio/wafie/discovery/pkg/discovery/endpointslice"
	"github.com/wafieio/wafie/discovery/pkg/discovery/ingress"
	applogger "github.com/wafieio/wafie/logger"
	"go.uber.org/zap"
)

func init() {
	startCmd.PersistentFlags().StringP(
		"ingress-type",
		"i",
		"ingress",
		fmt.Sprintf("one of %s|%s|%s",
			ingress.VsIngressType,
			ingress.K8sIngressType,
			ingress.RouteIngressType),
	)
	startCmd.PersistentFlags().StringP("api-addr", "a", "http://localhost:8080", "API address")
	viper.BindPFlag("ingress-type", startCmd.PersistentFlags().Lookup("ingress-type"))
	viper.BindPFlag("api-addr", startCmd.PersistentFlags().Lookup("api-addr"))
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "start wafie discovery agent",
	Run: func(cmd *cobra.Command, args []string) {
		// start health check server
		hsrv.NewHealthCheckServer(
			":8081", viper.GetString("api-addr"),
		).Serve()
		// run ingress cache
		ingress.NewIngressCache(
			viper.GetString("ingress-type"),
			viper.GetString("api-addr"),
			applogger.NewLogger(),
		).Run()
		// run endpointslice cache
		endpointslice.NewCache(
			viper.GetString("api-addr"),
			applogger.NewLogger(),
		).Run()
		// handle interrupts
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		for {
			select {
			case s := <-sigCh:
				zap.S().Infof("signal: %s, shutting down", s)
				zap.S().Info("bye bye 👋")
				os.Exit(0)
			}
		}
	},
}
