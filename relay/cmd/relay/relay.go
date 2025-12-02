package relay

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	applogger "github.com/wafieio/wafie/logger"
	"github.com/wafieio/wafie/relay/pkg/apisrv"
	"github.com/wafieio/wafie/relay/pkg/relay"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	relayCmd.PersistentFlags().BoolP("logs-to-stdout", "l", false, "Print logs to stdout instead of file")
	viper.BindPFlag("logs-to-stdout", relayCmd.PersistentFlags().Lookup("logs-to-stdout"))
	startCmd.AddCommand(relayCmd)
}

var relayCmd = &cobra.Command{
	Use:   "relay-instance",
	Short: "start wafie relay instance",
	Run: func(cmd *cobra.Command, args []string) {
		logger := initLogger()
		socatRelay := relay.NewSocat(logger)
		// start relay api server
		apisrv.
			NewServer(logger, socatRelay).
			Start()
		shutdown(socatRelay)
	},
}

func initLogger() *zap.Logger {
	if viper.GetBool("logs-to-stdout") {
		return applogger.NewLogger()
	}
	return applogger.NewLoggerToFile()
}

func shutdown(s relay.Relay) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	gracefullyExit := func(s relay.Relay, sig os.Signal) {
		_, stop := s.Configure(nil)
		stop()
		logger.Info("shutting down, bye bye 👋", zap.String("signal", sig.String()))
		if s, ok := sig.(syscall.Signal); ok {
			os.Exit(128 + int(s))
		}
		os.Exit(1)
	}
	gracefullyExit(s, <-sigCh)
}
