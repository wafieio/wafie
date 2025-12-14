package main

import (
	"net"
	"os"

	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wafieio/wafie/logger"
	"github.com/wafieio/wafie/xproc/pkg/processor"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	rootCmd = &cobra.Command{
		Use: "xproc - Envoy External Processor Filter",
	}
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "start gRPC server",
		Run: func(cmd *cobra.Command, args []string) {
			runServer()
		},
	}
)

func init() {
	startCmd.PersistentFlags().StringP("api-addr", "a", "http://localhost:8080", "API address")
	startCmd.PersistentFlags().StringP("xproc-socket", "s",
		"/var/run/wafie/xproc/socket", "wafie ext proc socket")
	viper.BindPFlag("api-addr", startCmd.PersistentFlags().Lookup("api-addr"))
	viper.BindPFlag("xproc-socket", startCmd.PersistentFlags().Lookup("xproc-socket"))
	rootCmd.AddCommand(startCmd)
}

func runServer() {
	l := logger.NewLogger()
	apiAddr := viper.GetString("api-addr")
	socket := viper.GetString("xproc-socket")
	if err := os.RemoveAll(socket); err != nil {
		l.Error("failed to create listening unix socket", zap.String("socket", socket), zap.Error(err))
		os.Exit(1)
	}
	lis, err := net.Listen("unix", socket)
	if err != nil {
		l.Error("failed to listen: %v", zap.Error(err))
		os.Exit(1)
	}
	if err := os.Chmod(socket, 0666); err != nil {
		l.Error("failed to chmod for a socket", zap.String("socket", socket), zap.Error(err))
		os.Exit(1)
	}
	srv := grpc.NewServer()
	extproc.RegisterExternalProcessorServer(srv, processor.NewExternalProcessor(apiAddr, l))
	l.Info("wafie external processor server listening", zap.String("socket", socket))
	if err := srv.Serve(lis); err != nil {
		l.Error("failed to serve", zap.String("socket", socket), zap.Error(err))
	}
	defer os.Remove(socket)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
