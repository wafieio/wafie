package main

import (
	"github.com/Dimss/wafie/logger"
	"github.com/Dimss/wafie/xproc/pkg/processor"
	extproc "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

var (
	rootCmd = &cobra.Command{
		Use: "xproc - Envoy External Processor Filter",
	}
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "start gRPC server",
		Run: func(cmd *cobra.Command, args []string) {
			logger := logger.NewLogger()
			addr := viper.GetString("listen-addr")
			lis, err := net.Listen("tcp", addr)
			if err != nil {
				log.Fatalf("Failed to listen: %v", err)
			}
			s := grpc.NewServer()
			extproc.RegisterExternalProcessorServer(s, processor.NewExternalProcessor(logger))
			logger.Info("wafie external processor server listening", zap.String("address", addr))
			if err := s.Serve(lis); err != nil {
				log.Fatalf("Failed to serve: %v", err)
			}
		},
	}
)

func init() {
	startCmd.PersistentFlags().StringP("listen-addr", "l", ":50051", "listen address")
	viper.BindPFlag("listen-addr", startCmd.PersistentFlags().Lookup("listen-addr"))
	rootCmd.AddCommand(startCmd)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
