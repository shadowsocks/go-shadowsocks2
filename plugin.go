package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

var pluginCmd *exec.Cmd

func startPlugin(plugin string, pluginOpts string, v2rayPort int, ssPort int) (err error) {
	logf("starting plugin (%s) with option (%s)....", plugin, pluginOpts)
	logf("plugin (%s) will listen on %s:%d", plugin, listen, v2rayPort)
	return execPlugin(plugin, pluginOpts, listen, v2rayPort, forward, ssPort)
}

func killPlugin() {
	if pluginCmd != nil {
		pluginCmd.Process.Signal(syscall.SIGTERM)
		waitCh := make(chan struct{})
		go func() {
			pluginCmd.Wait()
			close(waitCh)
		}()
		timeout := time.After(3 * time.Second)
		select {
		case <-waitCh:
		case <-timeout:
			pluginCmd.Process.Kill()
		}
	}
}

func execPlugin(plugin string, pluginOpts string, listen string, v2rayPort int, forward string, ssPort int) (err error) {
	pluginFile := plugin
	if fileExists(plugin) {
		if !filepath.IsAbs(plugin) {
			pluginFile = "./" + plugin
		}
	} else {
		pluginFile, err = exec.LookPath(plugin)
		if err != nil {
			return err
		}
	}
	logH := newLogHelper("[" + plugin + "]: ")
	env := append(os.Environ(),
		fmt.Sprintf("SS_REMOTE_HOST=%s", listen),
		fmt.Sprintf("SS_REMOTE_PORT=%d", v2rayPort),
		fmt.Sprintf("SS_LOCAL_HOST=%s", forward),
		fmt.Sprintf("SS_LOCAL_PORT=%d", ssPort),
		"SS_PLUGIN_OPTIONS="+pluginOpts,
	)
	cmd := &exec.Cmd{
		Path:   pluginFile,
		Env:    env,
		Stdout: logH,
		Stderr: logH,
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	pluginCmd = cmd
	go func() {
		if err := cmd.Wait(); err != nil {
			logf("plugin exited (%v)\n", err)
			os.Exit(2)
		}
		logf("plugin exited\n")
		os.Exit(0)
	}()
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	port := fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
	l.Close()
	return port, nil
}
