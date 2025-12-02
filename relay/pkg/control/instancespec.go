package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/containernetworking/plugins/pkg/ns"
	healthv1 "github.com/wafieio/wafie/api/gen/grpc/health/v1"
	"github.com/wafieio/wafie/api/gen/grpc/health/v1/healthv1connect"
	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"github.com/wafieio/wafie/api/gen/wafie/v1/wafiev1connect"
	"github.com/wafieio/wafie/relay/pkg/apisrv"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	ContainerdCRISock = "unix:///run/containerd/containerd.sock" // Adjust for your specific runtime
	CRIoCRISock       = "unix:///var/run/crio/crio.sock"
)

type RelayInstanceSpec struct {
	containerId  string
	runtimeSock  string
	nodeName     string
	netnsPath    string
	logger       *zap.Logger
	apiAddr      string
	podName      string
	relayOptions *wv1.RelayOptions
}

func NewRelayInstanceSpec(containerId, podName, nodeName string, options *wv1.RelayOptions, logger *zap.Logger) (*RelayInstanceSpec, error) {
	var err error
	i := &RelayInstanceSpec{
		logger:       logger.With(zap.String("podName", podName)),
		nodeName:     nodeName,
		apiAddr:      fmt.Sprintf("http://127.0.0.1:%d", apisrv.ApiListeningPort),
		podName:      podName,
		relayOptions: options,
	}
	// set container id
	if i.containerId, i.runtimeSock, err = parseContainerId(containerId); err != nil {
		return nil, err
	}
	if err := i.discoverNetnsPath(); err != nil {
		return nil, err
	}
	// add netns to relay options
	// the netns will be used in relay instance
	// as a part of health check procedure
	i.relayOptions.Netns = i.netnsPath
	i.logger = logger.With(
		zap.String("podName", podName),
		zap.String("netNs", i.netnsPath),
		zap.String("containerId", containerId),
		zap.String("nodeName", nodeName),
	)
	return i, nil
}

// StartSpec idempotent method, will do nothing if instance already injected and running
// otherwise will clean up previous instance and start a new one
func (s *RelayInstanceSpec) StartSpec() error {
	s.logger.Debug("starting relay", zap.Any("relayOptions", s.relayOptions.String()))
	if !s.relayRunning() {
		if err := s.runRelayBinary(); err != nil {
			return err
		}
		// this irrational sleep is here because
		// I've no logic for waiting because
		// I'm not yet implemented the status endpoint
		// TODO: implement readiness endpoint instead
		time.Sleep(2 * time.Second)
	}
	return s.startRelay()
}

func (s *RelayInstanceSpec) StopSpec() error {
	s.logger.Debug("stopping relay")
	if !s.relayRunning() {
		return nil
	}
	_, err := wafiev1connect.NewRelayServiceClient(s.namespacedHttpClient(), s.apiAddr).
		StopRelay(context.Background(), connect.NewRequest(&wv1.StopRelayRequest{}))
	if err != nil {
		return err
	}
	return nil
}

func (s *RelayInstanceSpec) startRelay() error {
	if !s.relayRunning() {
		return nil
	}
	_, err := wafiev1connect.NewRelayServiceClient(s.namespacedHttpClient(), s.apiAddr).
		StartRelay(context.Background(),
			connect.NewRequest(&wv1.StartRelayRequest{
				Options: s.relayOptions,
			}),
		)
	if err != nil {
		return err
	}
	return nil
}

func (s *RelayInstanceSpec) runRelayBinary() error {
	var netNs ns.NetNS
	defer func(netNs ns.NetNS) {
		if netNs != nil {
			netNs.Close()
		}
	}(netNs)
	netNs, err := ns.GetNS(s.netnsPath)
	if err != nil {
		return err
	}
	return netNs.Do(func(_ ns.NetNS) error {
		s.logger.Info("network namespace set", zap.String("path", s.netnsPath))
		cmd := exec.Command(
			"/usr/local/bin/wafie-relay",
			"start", "relay-instance",
		)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		return cmd.Start()
	})
}

func (s *RelayInstanceSpec) namespacedHttpClient() *http.Client {
	dialer := &net.Dialer{}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				// Save current namespace (optional but safer)
				currentNS, err := os.Open("/proc/self/ns/net")
				if err != nil {
					return nil, err
				}
				defer currentNS.Close()
				// Switch to target namespace
				nsFile, err := os.Open(s.netnsPath)
				if err != nil {
					return nil, err
				}
				defer nsFile.Close()
				err = unix.Setns(int(nsFile.Fd()), unix.CLONE_NEWNET)
				if err != nil {
					return nil, err
				}
				// Restore original namespace when done
				defer unix.Setns(int(currentNS.Fd()), unix.CLONE_NEWNET)
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
}

func (s *RelayInstanceSpec) relayRunning() (isRunning bool) {
	relayHealthCheck := healthv1connect.NewHealthClient(s.namespacedHttpClient(), s.apiAddr)
	resp, err := relayHealthCheck.Check(context.Background(), connect.NewRequest(&healthv1.HealthCheckRequest{}))
	// if relay no running, expecting to get CodeUnavailable (ECONNREFUSED)
	if connect.CodeOf(err) == connect.CodeUnavailable {
		return false
	}
	// TODO: need a way to monitor an errors and reactively fix them
	// in case of any error do nothing, s.e return true
	if err != nil {
		s.logger.Error("health check failed", zap.Error(err))
		return true
	}
	if resp.Msg.GetStatus() == healthv1.HealthCheckResponse_SERVING {
		return true
	}
	return false
}

func (s *RelayInstanceSpec) discoverNetnsPath() error {
	conn, err := grpc.NewClient(
		s.runtimeSock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %v", err)
	}
	defer conn.Close()
	client := runtimeapi.NewRuntimeServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request := &runtimeapi.ContainerStatusRequest{
		ContainerId: s.containerId,
		Verbose:     true,
	}
	response, err := client.ContainerStatus(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to get container status: %v\n", err)
	}
	if s.netnsPath, err = getContainerNetworkNs(response); err != nil {
		return err
	}
	return nil
}

func getContainerNetworkNs(containerStatusResponse *runtimeapi.ContainerStatusResponse) (string, error) {
	infoMap := make(map[string]interface{})
	if _, ok := containerStatusResponse.Info["info"]; !ok {
		return "", fmt.Errorf("info not found in response")
	}
	if err := json.Unmarshal([]byte(containerStatusResponse.Info["info"]), &infoMap); err != nil {
		fmt.Printf("Failed to unmarshal info: %v\n", err)
	}
	nsUnstructured := infoMap["runtimeSpec"].(map[string]interface{})["linux"].(map[string]interface{})["namespaces"].([]interface{})
	res, err := json.Marshal(nsUnstructured)
	if err != nil {
		return "", err
	}
	type namespace struct {
		Key  string `json:"type"`
		Path string `json:"path,omitempty"`
	}
	var namespaces []namespace
	err = json.Unmarshal(res, &namespaces)
	if err != nil {
		fmt.Printf("failed to unmarshal namespaces: %v\n", err)
	}
	for _, ns := range namespaces {
		if ns.Key == "network" {
			return ns.Path, nil
		}
	}
	return "", fmt.Errorf("failed to find network namespace")
}

func parseContainerId(containerId string) (id string, runtimeSock string, err error) {
	slice := strings.Split(containerId, "://")
	if len(slice) != 2 {
		return "", "", fmt.Errorf("unable to parse container id")
	}
	if slice[0] == "cri-o" {
		return slice[1], CRIoCRISock, nil
	}
	if slice[0] == "containerd" {
		return slice[1], ContainerdCRISock, nil
	}
	return "", "", fmt.Errorf("unable to detect container runtime")

}
