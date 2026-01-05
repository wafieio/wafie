package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:   "wafie-api-server",
		Short: "WAFie API Server",
	}
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(func() {
		// setup logging
		//config := zap.NewDevelopmentConfig()
		//config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		//config.EncoderConfig.TimeKey = "timestamp"
		//config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		//logger, _ := config.Build()
		//zap.ReplaceGlobals(logger)
		// setup viper
		viper.AutomaticEnv()
		viper.SetEnvPrefix("CWAF_API_SERVER")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	})
}
