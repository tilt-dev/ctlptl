package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	klog "k8s.io/klog/v2"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Uses the DockerForMac GUI protocol to control DockerForMac.
//
// There isn't an off-the-shelf library or documented protocol we can use
// for this, so we do the best we can.
type DockerForMacClient struct {
	httpClient HTTPClient
	socketPath string
}

func NewDockerForMacClient() (DockerForMacClient, error) {
	homedir, err := homedir.Dir()
	if err != nil {
		return DockerForMacClient{}, err
	}

	socketPath := filepath.Join(homedir, "Library/Containers/com.docker.docker/Data/gui-api.sock")
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
	return DockerForMacClient{
		httpClient: httpClient,
		socketPath: socketPath,
	}, nil
}

func (c DockerForMacClient) start(ctx context.Context) error {
	_, err := os.Stat("/Applications/Docker.app")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Please install Docker for Desktop: https://www.docker.com/products/docker-desktop")
		}
		return err
	}

	cmd := exec.Command("open", "/Applications/Docker.app")
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "starting Docker")
	}
	return err
}

func (c DockerForMacClient) resetK8s(ctx context.Context) error {
	klog.V(7).Infof("POST %s /kubernetes/reset\n", c.socketPath)
	req, err := http.NewRequest("POST", "http://localhost/kubernetes/reset", nil)
	if err != nil {
		return errors.Wrap(err, "reset d4m kubernetes")
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "reset d4m kubernetes")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("reset d4m kubernetes: status code %d", resp.StatusCode)
	}
	return nil
}

func (c DockerForMacClient) SettingsValues(ctx context.Context) (interface{}, error) {
	s, err := c.settings(ctx)
	if err != nil {
		return nil, err
	}
	return c.settingsForWrite(s), nil
}

func (c DockerForMacClient) writeSettings(ctx context.Context, settings map[string]interface{}) error {
	klog.V(7).Infof("POST %s /settings\n", c.socketPath)
	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(c.settingsForWrite(settings))
	if err != nil {
		return errors.Wrap(err, "writing d4m settings")
	}

	klog.V(8).Infof("Request body: %s\n", buf.String())
	req, err := http.NewRequest("POST", "http://localhost/settings", buf)
	if err != nil {
		return errors.Wrap(err, "writing d4m settings")
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "writing d4m settings")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("writing d4m settings: status code %d", resp.StatusCode)
	}
	return nil
}

func (c DockerForMacClient) settings(ctx context.Context) (map[string]interface{}, error) {
	klog.V(7).Infof("GET %s /settings\n", c.socketPath)
	req, err := http.NewRequest("GET", "http://localhost/settings", nil)
	if err != nil {
		return nil, errors.Wrap(err, "reading d4m settings")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "reading d4m settings")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading d4m settings: status code %d", resp.StatusCode)
	}

	settings := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&settings)
	if err != nil {
		return nil, errors.Wrap(err, "reading d4m settings")
	}
	klog.V(8).Infof("Response body: %+v\n", settings)
	return settings, nil
}

func (c DockerForMacClient) lookupMapAt(settings map[string]interface{}, key string) (map[string]interface{}, error) {
	parts := strings.Split(key, ".")
	current := settings
	for i, part := range parts {
		var ok bool
		current, ok = current[part].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected map at DockerForMac setting %s, got: %T",
				strings.Join(parts[:i+1], "."), current[part])
		}
	}
	return current, nil
}

func (c DockerForMacClient) setK8sEnabled(settings map[string]interface{}, newVal bool) (changed bool, err error) {
	enabledSetting, err := c.lookupMapAt(settings, "vm.kubernetes.enabled")
	if err != nil {
		return false, err
	}

	isEnabled, ok := enabledSetting["value"].(bool)
	if !ok {
		return false, fmt.Errorf("expected bool at DockerForMac setting vm.kubernetes.enabled.value, got: %T",
			enabledSetting["value"])
	}

	if isEnabled == newVal {
		return false, nil
	}
	enabledSetting["value"] = newVal
	return true, nil
}

func (c DockerForMacClient) ensureMinCPU(settings map[string]interface{}, desired int) (changed bool, err error) {
	cpusSetting, err := c.lookupMapAt(settings, "vm.resources.cpus")
	if err != nil {
		return false, err
	}

	value, ok := cpusSetting["value"].(float64)
	if !ok {
		return false, fmt.Errorf("expected number at DockerForMac setting vm.resources.cpus.value, got: %T",
			cpusSetting["value"])
	}
	max, ok := cpusSetting["max"].(float64)
	if !ok {
		return false, fmt.Errorf("expected number at DockerForMac setting vm.resources.cpus.max, got: %T",
			cpusSetting["max"])
	}

	if desired > int(max) {
		return false, fmt.Errorf("desired cpus (%d) greater than max allowed (%d)", desired, int(max))
	}

	if desired <= int(value) {
		return false, nil
	}

	cpusSetting["value"] = desired
	return true, nil
}

func (c DockerForMacClient) settingsForWrite(settings interface{}) interface{} {
	settingsMap, ok := settings.(map[string]interface{})
	if !ok {
		return settings
	}

	_, hasLocked := settingsMap["locked"]
	value, hasValue := settingsMap["value"]
	if hasLocked && hasValue {
		return value
	}

	if hasLocked && len(settingsMap) == 1 {
		return nil
	}

	_, hasLocks := settingsMap["locks"]
	json, hasJSON := settingsMap["json"]
	if hasLocks && hasJSON {
		return json
	}

	for key, value := range settingsMap {
		newVal := c.settingsForWrite(value)
		if newVal != nil {
			settingsMap[key] = newVal
		} else {
			delete(settingsMap, key)
		}
	}

	return settings
}
