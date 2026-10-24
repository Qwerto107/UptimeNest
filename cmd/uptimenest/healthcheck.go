package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// healthcheck 仅探测正在运行的服务，不打开数据库或启动监控任务。
func healthcheck() error {
	address := os.Getenv("UPTIMENEST_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8090"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	} else if host == "::" {
		host = "::1"
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
	response, err := client.Get("http://" + net.JoinHostPort(host, port) + "/api/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("健康检查返回 HTTP %d", response.StatusCode)
	}
	return nil
}
