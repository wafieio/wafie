package gateway

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:   "appsecgw",
		Short: "WAFie Gateway Control Plane gRPC Server",
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
		viper.AutomaticEnv()
		viper.SetEnvPrefix("WAFIE_APPSECGW_SERVER")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	})
}
