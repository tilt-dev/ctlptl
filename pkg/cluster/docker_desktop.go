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
	"runtime"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	klog "k8s.io/klog/v2"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Uses the DockerDesktop GUI protocol to control DockerDesktop.
//
// There isn't an off-the-shelf library or documented protocol we can use
// for this, so we do the best we can.
type DockerDesktopClient struct {
	httpClient HTTPClient
}

func NewDockerDesktopClient() (DockerDesktopClient, error) {
	socketPaths, err := dockerDesktopSocketPaths()
	if err != nil {
		return DockerDesktopClient{}, err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var lastErr error

				// Different versions of docker use different socket paths,
				// so return all of them and connect to the first one that
				// accepts a TCP dial.
				for _, socketPath := range socketPaths {
					conn, err := dialDockerDesktop(socketPath)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, lastErr
			},
		},
	}
	return DockerDesktopClient{
		httpClient: httpClient,
	}, nil
}

func (c DockerDesktopClient) Open(ctx context.Context) error {
	var err error
	switch runtime.GOOS {

	case "windows":
		return fmt.Errorf("Cannot auto-start Docker Desktop on Windows")

	case "darwin":
		_, err = os.Stat("/Applications/Docker.app")
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Please install Docker for Desktop: https://www.docker.com/products/docker-desktop")
			}
			return err
		}
		cmd := exec.Command("open", "/Applications/Docker.app")
		err = cmd.Run()

	case "linux":
		cmd := exec.Command("systemctl", "--user", "start", "docker-desktop")
		err = cmd.Run()
	}

	if err != nil {
		return errors.Wrap(err, "starting Docker")
	}
	return nil
}

func (c DockerDesktopClient) Quit(ctx context.Context) error {
	var err error
	switch runtime.GOOS {
	case "windows":
		return fmt.Errorf("Cannot quit Docker Desktop on Windows")

	case "darwin":
		cmd := exec.Command("osascript", "-e", `quit app "Docker"`)
		err = cmd.Run()

	case "linux":
		cmd := exec.Command("systemctl", "--user", "stop", "docker-desktop")
		err = cmd.Run()
	}

	if err != nil {
		return errors.Wrap(err, "quitting Docker")
	}
	return nil
}

func (c DockerDesktopClient) ResetCluster(ctx context.Context) error {
	klog.V(7).Infof("POST /kubernetes/reset\n")
	req, err := http.NewRequest("POST", "http://localhost/kubernetes/reset", nil)
	if err != nil {
		return errors.Wrap(err, "reset docker-desktop kubernetes")
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "reset docker-desktop kubernetes")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("reset docker-desktop kubernetes: status code %d", resp.StatusCode)
	}
	return nil
}

func (c DockerDesktopClient) SettingsValues(ctx context.Context) (interface{}, error) {
	s, err := c.settings(ctx)
	if err != nil {
		return nil, err
	}
	return c.settingsForWrite(s), nil
}

func (c DockerDesktopClient) SetSettingValue(ctx context.Context, key, newValue string) error {
	settings, err := c.settings(ctx)
	if err != nil {
		return err
	}

	changed, err := c.applySet(settings, key, newValue)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return c.writeSettings(ctx, settings)
}

// Returns true if the value changed, false if the value is unchanged.
// Returns an error if not able to set.
func (c DockerDesktopClient) applySet(settings map[string]interface{}, key, newValue string) (bool, error) {
	parts := strings.Split(key, ".")
	if len(parts) <= 1 {
		return false, fmt.Errorf("key cannot be set: %s", key)
	}

	parentKey := strings.Join(parts[:len(parts)-1], ".")
	childKey := parts[len(parts)-1]
	parentSpec, err := c.lookupMapAt(settings, parentKey)
	if err != nil {
		return false, err
	}

	// In Docker Desktop, a boolean setting can be stored in one of two formats:
	//
	// {"kubernetes": {"enabled": true}}
	// {"kubernetes": {"enabled": {"value": true}}}
	//
	// To resolve this problem, we create some intermediate variables:
	// v - the value that we're replacing
	// vParent - the map owning the value we're replacing
	// vParentKey - the key where v lives in vParent
	v, ok := parentSpec[childKey]
	if !ok {
		return false, fmt.Errorf("nothing found at DockerDesktop setting %q", key)
	}

	vParent := parentSpec
	vParentKey := childKey
	childMap, isMap := v.(map[string]interface{})
	if isMap {
		v = childMap["value"]
		vParent = childMap
		vParentKey = "value"
	}

	switch v := v.(type) {
	case bool:
		if newValue == "true" {
			vParent[vParentKey] = true
			return !v, nil
		} else if newValue == "false" {
			vParent[vParentKey] = false
			return v, nil
		}

		return false, fmt.Errorf("expected bool for setting %q, got: %s", key, newValue)

	case float64:
		newValFloat, err := strconv.ParseFloat(newValue, 64)
		if err != nil {
			return false, fmt.Errorf("expected number for setting %q, got: %s. Error: %v", key, newValue, err)
		}

		max, ok := vParent["max"].(float64)
		if ok && newValFloat > max {
			return false, fmt.Errorf("setting value %q: %s greater than max allowed (%f)", key, newValue, max)
		}
		min, ok := vParent["min"].(float64)
		if ok && newValFloat < min {
			return false, fmt.Errorf("setting value %q: %s less than min allowed (%f)", key, newValue, min)
		}

		if newValFloat != v {
			vParent[vParentKey] = newValFloat
			return true, nil
		}
		return false, nil
	case string:
		if newValue != v {
			vParent[vParentKey] = newValue
			return true, nil
		}
		return false, nil
	default:
		if key == "vm.fileSharing" {
			pathSpec := []map[string]interface{}{}
			paths := strings.Split(newValue, ",")
			for _, path := range paths {
				pathSpec = append(pathSpec, map[string]interface{}{"path": path, "cached": false})
			}

			vParent[vParentKey] = pathSpec

			// Don't bother trying to optimize this.
			return true, nil
		}
	}

	return false, fmt.Errorf("Cannot set key: %q", key)
}

func (c DockerDesktopClient) writeSettings(ctx context.Context, settings map[string]interface{}) error {
	klog.V(7).Infof("POST /settings\n")
	buf := bytes.NewBuffer(nil)
	err := json.NewEncoder(buf).Encode(c.settingsForWrite(settings))
	if err != nil {
		return errors.Wrap(err, "writing docker-desktop settings")
	}

	klog.V(8).Infof("Request body: %s\n", buf.String())
	req, err := http.NewRequest("POST", "http://localhost/settings", buf)
	if err != nil {
		return errors.Wrap(err, "writing docker-desktop settings")
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "writing docker-desktop settings")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("writing docker-desktop settings: status code %d", resp.StatusCode)
	}
	return nil
}

func (c DockerDesktopClient) settings(ctx context.Context) (map[string]interface{}, error) {
	klog.V(7).Infof("GET /settings\n")
	req, err := http.NewRequest("GET", "http://localhost/settings", nil)
	if err != nil {
		return nil, errors.Wrap(err, "reading docker-desktop settings")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Docker Desktop. "+
			"Please ensure Docker is installed and up to date.\n  (caused by: %v)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading docker-desktop settings: status code %d", resp.StatusCode)
	}

	settings := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&settings)
	if err != nil {
		return nil, errors.Wrap(err, "reading docker settings")
	}
	klog.V(8).Infof("Response body: %+v\n", settings)
	return settings, nil
}

func (c DockerDesktopClient) lookupMapAt(settings map[string]interface{}, key string) (map[string]interface{}, error) {
	parts := strings.Split(key, ".")
	current := settings
	for i, part := range parts {
		var ok bool
		val := current[part]
		current, ok = val.(map[string]interface{})
		if !ok {
			if val == nil {
				return nil, fmt.Errorf("nothing found at DockerDesktop setting %q",
					strings.Join(parts[:i+1], "."))
			}
			return nil, fmt.Errorf("expected map at DockerDesktop setting %q, got: %T",
				strings.Join(parts[:i+1], "."), val)
		}
	}
	return current, nil
}

func (c DockerDesktopClient) setK8sEnabled(settings map[string]interface{}, newVal bool) (changed bool, err error) {
	return c.applySet(settings, "vm.kubernetes.enabled", fmt.Sprintf("%v", newVal))
}

func (c DockerDesktopClient) ensureMinCPU(settings map[string]interface{}, desired int) (changed bool, err error) {
	cpusSetting, err := c.lookupMapAt(settings, "vm.resources.cpus")
	if err != nil {
		return false, err
	}

	value, ok := cpusSetting["value"].(float64)
	if !ok {
		return false, fmt.Errorf("expected number at DockerDesktop setting vm.resources.cpus.value, got: %T",
			cpusSetting["value"])
	}
	max, ok := cpusSetting["max"].(float64)
	if !ok {
		return false, fmt.Errorf("expected number at DockerDesktop setting vm.resources.cpus.max, got: %T",
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

func (c DockerDesktopClient) settingsForWrite(settings interface{}) interface{} {
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
