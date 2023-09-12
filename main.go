package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	listenAddrFlag = flag.String("listen", "0.0.0.0:80", "Listen addr")
	backendCidr    = []*net.IPNet{}
)

const (
	acmeChallengePath = "/.well-known/acme-challenge"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		flag.Func("backend-cidr", "Specify backend cidr ranges", func(arg string) error {
			_, ipnet, err := net.ParseCIDR(arg)
			if err != nil {
				return err
			}
			backendCidr = append(backendCidr, ipnet)
			return nil
		})
		flag.Parse()

		transport := &http.Transport{
			DialContext:           createDialContext(backendCidr),
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		httpClient := &http.Client{
			Transport: transport,
		}

		redirectServer := &http.Server{
			Addr:    *listenAddrFlag,
			Handler: http.HandlerFunc(createHandler(httpClient, logger)),
		}

		logger.Info("listening", "listenAddr", *listenAddrFlag)
		if err := redirectServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err, "listenAddr", *listenAddrFlag)
		}
	}()

	<-sigs
	logger.Info("mlik stopped")
}

func createHandler(client httpClient, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, acmeChallengePath) {
			resp, err := client.Do(r)
			if err != nil {
				logger.Error("failed to send request to upstream", "err", err)
				r.Body.Close()
				http.Error(w, "Upstream error", http.StatusBadGateway)
				return
			}
			for key, valList := range resp.Header {
				for _, headerValue := range valList {
					w.Header().Add(key, headerValue)
				}
			}
			if resp.Body != nil {
				if _, err := io.Copy(w, resp.Body); err != nil {
					logger.Error("failed to write response from upstream to client", "err", err)
				}
			}
			w.WriteHeader(resp.StatusCode)
		}
		redirectUrl := r.URL
		redirectUrl.Scheme = "https"
		w.Header().Set("Location", redirectUrl.String())
		w.WriteHeader(http.StatusPermanentRedirect)
	}
}

func checkBackend(address string, backendCidrs []*net.IPNet) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ipaddress := net.ParseIP(host)
	if ipaddress == nil {
		return fmt.Errorf("%s is not a valid IP address", host)
	}
	for _, cidr := range backendCidrs {
		if cidr.Contains(ipaddress) {
			return nil
		}
	}
	return fmt.Errorf("%s is not within an allowed backend cidr", address)
}

func createDialContext(backendCidrs []*net.IPNet) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{
			Timeout: 5 * time.Second,
			Control: func(network string, address string, c syscall.RawConn) error {
				if err := checkBackend(address, backendCidrs); err != nil {
					return err
				}
				return nil
			},
		}
		// Always dial via IPv6
		return dialer.DialContext(ctx, "tcp6", addr)
	}
}
