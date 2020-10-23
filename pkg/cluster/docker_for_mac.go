package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	klog "k8s.io/klog/v2"
)

type d4mSettings map[string]interface{}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Uses the DockerForMac GUI protocol to control DockerForMac.
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

func (c DockerForMacClient) ensureK8sEnabled(settings map[string]interface{}) (changed bool, err error) {
	vmSetting, ok := settings["vm"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("missing vm setting")
	}

	k8sSetting, ok := vmSetting["kubernetes"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("missing kubernetes setting")
	}

	enabledSetting, ok := k8sSetting["enabled"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("missing kubernetes enabled setting")
	}

	isEnabled, ok := enabledSetting["value"].(bool)
	if !ok {
		return false, fmt.Errorf("missing kubernetes enabled setting")
	}

	if isEnabled {
		return false, nil
	}
	enabledSetting["value"] = true
	return true, nil
}

func (c DockerForMacClient) ensureMinCPU(settings map[string]interface{}, desired int) (changed bool, err error) {
	vmSetting, ok := settings["vm"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("missing vm setting")
	}

	resourcesSetting, ok := vmSetting["resources"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("missing resources setting")
	}

	cpusSetting, ok := resourcesSetting["cpus"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("missing resources cpus setting")
	}

	value, ok := cpusSetting["value"].(float64)
	if !ok {
		return false, fmt.Errorf("missing resources cpu values")
	}
	max, ok := cpusSetting["max"].(float64)
	if !ok {
		return false, fmt.Errorf("missing resources cpu max")
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
