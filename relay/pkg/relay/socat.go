//go:build linux

package relay

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"go.uber.org/zap"
)

type SocatRelay struct {
	cmd                *exec.Cmd
	command            string
	args               []string
	logger             *zap.Logger
	options            *wv1.RelayOptions
	startHealthMonitor chan struct{}
	stopHealthMonitor  chan struct{}
}

func NewSocat(logger *zap.Logger) *SocatRelay {
	socatRelay := &SocatRelay{
		logger:             logger,
		startHealthMonitor: make(chan struct{}),
		stopHealthMonitor:  make(chan struct{}),
	}
	socatRelay.runProxyHealthMonitor()
	return socatRelay
}

func (r *SocatRelay) shouldRestart(cfgOptions *wv1.RelayOptions) bool {
	if cfgOptions == nil {
		return false
	}
	return r.options.ProxyFqdn != cfgOptions.ProxyFqdn ||
		r.options.AppContainerPort != cfgOptions.AppContainerPort ||
		r.options.RelayPort != cfgOptions.RelayPort ||
		r.options.ProxyListeningPort != cfgOptions.ProxyListeningPort
}

func (r *SocatRelay) netNsOk() (ok bool) {
	_, err := os.Stat(r.options.Netns)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		r.logger.Debug("error stat netns file", zap.String("netns", r.options.Netns), zap.Error(err))
		return false
	}
	return true
}

func (r *SocatRelay) proxyOk() (ok bool) {
	address := fmt.Sprintf("%s:%s", r.options.ProxyIp, r.options.ProxyListeningPort)
	pingMaxAttempts := 2
	for attempt := 0; attempt < pingMaxAttempts; attempt++ {
		time.Sleep(1 * time.Second)
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if conn != nil {
			if err := conn.Close(); err != nil {
				r.logger.Debug("error closing open connection", zap.String("address", address), zap.Error(err))
			}
		}
		if err == nil {
			return true
		}
		r.logger.Debug("proxy ping failed", zap.String("address", address), zap.Error(err))
	}
	return false
}

func (r *SocatRelay) runProxyHealthMonitor() {
	r.logger.Info("starting proxy health monitor")
	pingInterval := time.NewTicker(2 * time.Second)
	healthMonitorActive := true // by default
	defer r.logger.Debug("proxy health monitor terminated")
	go func() {
		for {
			select {
			case <-pingInterval.C:
				if r.options != nil && r.options.ProxyIp != "" && healthMonitorActive {
					if !r.netNsOk() {
						r.logger.Debug("netns removed, terminating")
						r.stopInternal()
						os.Exit(0)
					}

					if !r.proxyOk() {
						r.stopInternal()  // stop the proxy
						r.setProxyIp()    // lookup for the proxy IP
						r.startInternal() // start proxy
					}

				}
			case <-r.startHealthMonitor:
				r.logger.Info("starting health monitor")
				healthMonitorActive = true
			case <-r.stopHealthMonitor:
				r.logger.Info("terminating health monitor")
				healthMonitorActive = false
			}
		}
	}()
}

func (r *SocatRelay) Configure(cfgOptions *wv1.RelayOptions) (StartRelayFunc, StopRelayFunc) {
	// if options nil, meaning fresh start
	// in any case if proxy ip is not set, configure it
	if r.options == nil {
		r.options = cfgOptions
		r.setProxyIp()
		return r.start, r.stop
	}
	if r.options.ProxyIp == "" {
		r.options = cfgOptions
		r.setProxyIp()
		return r.start, r.stop
	}
	// if options already set, meaning relay already running
	// if start options changed from previous start,
	// the options must re-initiated
	// and relay must be restarted
	if r.shouldRestart(cfgOptions) {
		r.options = cfgOptions
		r.setProxyIp()
		r.stop()
	}
	return r.start, r.stop
}

func (r *SocatRelay) setProxyIp() {
	if r.options != nil && r.options.ProxyFqdn != "" {
		ips, err := net.LookupHost(r.options.ProxyFqdn)
		if err != nil {
			r.logger.Error("failed to set proxy ip", zap.Error(err), zap.String("proxyFqdn", r.options.ProxyFqdn))
			return
		}
		if len(ips) == 0 {
			r.logger.Error("empty IPs for", zap.String("proxyFqdn", r.options.ProxyFqdn))
			return
		}
		if len(ips) == 1 {
			r.options.ProxyIp = ips[0]
			return
		}
		rand.NewSource(time.Now().UnixNano())
		// the r.options.ProxyFqdn is usually K8s headless svc
		// which has behind multiple A records (pods IPs)
		// thus, I am just implementing
		// simple client side load balancing
		r.options.ProxyIp = ips[rand.Intn(len(ips))]
		r.logger.Info("discovered proxy ip", zap.String("proxyIp", r.options.ProxyIp))
	} else {
		r.logger.Debug("empty relay options, can not make dns lookup")
	}
}

func (r *SocatRelay) activateHealthMonitor() {
	r.startHealthMonitor <- struct{}{}
}

func (r *SocatRelay) deactivateHealthMonitor() {
	r.stopHealthMonitor <- struct{}{}
}

func (r *SocatRelay) socatRunning() bool {
	if r.cmd == nil || r.cmd.Process == nil {
		return false
	}
	if err := r.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		r.logger.Debug("signal 0 result", zap.Error(err))
		return false
	}
	return true
}

func (r *SocatRelay) startInternal() {
	if r.socatRunning() {
		r.logger.Info("socat already running")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cmd = exec.CommandContext(ctx,
		"socat",
		"-d",
		fmt.Sprintf("TCP-LISTEN:%s,"+
			"reuseaddr,fork,backlog=2048,rcvbuf=262144,sndbuf=262144,keepalive,nodelay,quickack",
			r.options.RelayPort),
		fmt.Sprintf("TCP:%s:%s,"+
			"rcvbuf=262144,sndbuf=262144,keepalive,nodelay,quickack,connect-timeout=3",
			r.options.ProxyIp, r.options.ProxyListeningPort),
	)
	r.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
		Pgid:    0,    // Use process ID as group ID
	}
	go func() {
		r.setupLogs()
		if err := r.cmd.Start(); err != nil {
			r.logger.Error("socat start error", zap.Error(err))
			cancel()
		}
		pgid, err := syscall.Getpgid(r.cmd.Process.Pid)
		if err != nil {
			panic(err)
		}
		r.logger.Debug("pgid", zap.Int("pgid", pgid))
		if err := r.setupNetwork(); err != nil {
			r.logger.Error("failed to setup network rules", zap.Error(err))
		}
		if err := r.cmd.Wait(); err != nil {
			r.logger.Error("socat run error", zap.Error(err))
		}
		r.logger.Debug("socat started successfully", zap.Int("pid", r.cmd.Process.Pid))
	}()
}

func (r *SocatRelay) start() {
	r.activateHealthMonitor()
	r.startInternal()
}

func (r *SocatRelay) setupNetwork() error {
	return ProgramNft(AddOp, r.options)
}

func (r *SocatRelay) stopInternal() {
	if r.cmd == nil || r.cmd.Process == nil {
		// un-program nft
		_ = ProgramNft(DeleteOp, r.options)
	}
	pid := r.cmd.Process.Pid
	// Kill the entire process group
	if err := syscall.Kill(-r.cmd.Process.Pid, syscall.SIGTERM); err != nil {
		fmt.Printf("Failed to send SIGTERM to process group: %v\n", err)
	}
	// Wait for graceful shutdown
	done := make(chan error, 1)
	go func() {
		done <- r.cmd.Wait()
	}()
	select {
	case err := <-done:
		fmt.Println("Socat stopped gracefully")
		fmt.Println(err)
	case <-time.After(5 * time.Second):
		fmt.Println("Timeout reached, force killing process group")
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			fmt.Printf("Failed to send SIGKILL: %v\n", err)
		}
	}
	// un-program nft
	_ = ProgramNft(DeleteOp, r.options)
}
func (r *SocatRelay) stop() {
	r.deactivateHealthMonitor()
	r.stopInternal()
}

func (r *SocatRelay) Status() {}

func (r *SocatRelay) setupLogs() {
	stdout, _ := r.cmd.StdoutPipe()
	stderr, _ := r.cmd.StderrPipe()
	go readProgramOutput(stdout)
	go readProgramOutput(stderr)
}

func readProgramOutput(readCloser io.ReadCloser) {
	_, err := io.Copy(log.Writer(), readCloser)
	if err != nil {
		log.Printf("error: %v", err)
	}
}
