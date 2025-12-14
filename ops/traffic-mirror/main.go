package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type RequestDump struct {
	Proto   string              `json:"proto"`
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body,omitempty"` // omitempty to exclude if body is empty
}

func init() {
	startCmd.PersistentFlags().IntP("port", "", 8080, "listening port")
	viper.BindPFlag("port", startCmd.PersistentFlags().Lookup("port"))
	rootCmd.AddCommand(startCmd)
}

var rootCmd = &cobra.Command{
	Use:   "traffic-mirror",
	Short: "Wafie Traffic Mirror server",
}
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts traffic mirror server",
	Run: func(cmd *cobra.Command, args []string) {
		http.HandleFunc("/", handler)
		listenAddr := fmt.Sprintf(":%d", viper.GetInt("port"))
		log.Printf("starting traffic mirror server on port %s\n", listenAddr)
		go func() {
			log.Fatal(http.ListenAndServe(listenAddr, nil))
		}()
		// handle interrupts
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		s := <-sigCh
		log.Printf("signal received: %s, shutting down\n", s.String())
		log.Println("bye bye 👋")
		os.Exit(0)
	},
}

// handler is the function that processes incoming HTTP requests.
func handler(w http.ResponseWriter, r *http.Request) {
	dump := RequestDump{
		Proto:   r.Proto,
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: r.Header,
	}
	dump.Headers["Host"] = []string{r.Host} // add host header, since Go http server won't Host header in Header map
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		dump.Body = string(bodyBytes)
	}
	// Marshal the struct to JSON
	jsonBytes, err := json.MarshalIndent(dump, "", "  ") // Use MarshalIndent for pretty-printing
	if err != nil {
		http.Error(w, "Error marshaling JSON", http.StatusInternalServerError)
		return
	}
	log.Println(string(jsonBytes))
	_, _ = w.Write(jsonBytes)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
