package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoOp(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	settings, err := f.d4m.settings(ctx)
	require.NoError(t, err)

	err = f.d4m.writeSettings(ctx, settings)
	require.NoError(t, err)

	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(postSettingsJSONV2)))
}

func TestEnableKubernetesV1(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	f.v = ddProtocolV1

	ctx := context.Background()
	settings, err := f.d4m.settings(ctx)
	require.NoError(t, err)

	changed, err := f.d4m.setK8sEnabled(settings, true)
	assert.True(t, changed)
	require.NoError(t, err)

	err = f.d4m.writeSettings(ctx, settings)
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV1,
		`"kubernetes":{"enabled":false`,
		`"kubernetes":{"enabled":true`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))
}

func TestEnableKubernetesV2(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	settings, err := f.d4m.settings(ctx)
	require.NoError(t, err)

	changed, err := f.d4m.setK8sEnabled(settings, true)
	assert.True(t, changed)
	require.NoError(t, err)

	err = f.d4m.writeSettings(ctx, settings)
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV2,
		`"kubernetes":{"enabled":{"locked":false,"value":false`,
		`"kubernetes":{"enabled":{"locked":false,"value":true`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))
}

func TestMinCPUs(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	settings, err := f.d4m.settings(ctx)
	require.NoError(t, err)

	changed, err := f.d4m.ensureMinCPU(settings, 4)
	assert.True(t, changed)
	require.NoError(t, err)

	err = f.d4m.writeSettings(ctx, settings)
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV2,
		`"cpus":{"max":8,"min":1,"value":2}`,
		`"cpus":{"max":8,"min":1,"value":4}`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))
}

func TestMaxCPUs(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	settings, err := f.d4m.settings(ctx)
	require.NoError(t, err)

	changed, err := f.d4m.ensureMinCPU(settings, 40)
	assert.False(t, changed)
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), "desired cpus (40) greater than max allowed (8)")
	}
}

func TestLookupMap(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	settings, err := f.d4m.settings(ctx)
	require.NoError(t, err)

	_, err = f.d4m.lookupMapAt(settings, "vm.kubernetes.honk")
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), `nothing found at DockerDesktop setting "vm.kubernetes.honk"`)
	}
}

func TestSetSettingValueInvalidKey(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.doesNotExist", "4")
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), `nothing found at DockerDesktop setting "vm.doesNotExist"`)
	}
}

func TestSetSettingValueInvalidSet(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.resources.cpus.value.doesNotExist", "4")
	if assert.Error(t, err) {
		assert.Equal(t, err.Error(), `expected map at DockerDesktop setting "vm.resources.cpus.value", got: float64`)
	}
}

func TestSetSettingValueFloat(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.resources.cpus", "4")
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV2,
		`"cpus":{"max":8,"min":1,"value":2}`,
		`"cpus":{"max":8,"min":1,"value":4}`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))

	f.postSettings = nil
	err = f.d4m.SetSettingValue(ctx, "vm.resources.cpus", "2")
	require.NoError(t, err)
	assert.Nil(t, f.postSettings)
}

func TestSetSettingValueFloatLimit(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.resources.cpus", "100")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), `setting value "vm.resources.cpus": 100 greater than max allowed`)
	}
	err = f.d4m.SetSettingValue(ctx, "vm.resources.cpus", "0")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), `setting value "vm.resources.cpus": 0 less than min allowed`)
	}
}

func TestSetSettingValueBoolV1(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()
	f.v = ddProtocolV1

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.kubernetes.enabled", "true")
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV1,
		`"enabled":false,`,
		`"enabled":true,`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))

	f.postSettings = nil
	err = f.d4m.SetSettingValue(ctx, "vm.kubernetes.enabled", "false")
	require.NoError(t, err)
	assert.Nil(t, f.postSettings)
}

func TestSetSettingValueBoolV2(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.kubernetes.enabled", "true")
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV2,
		`"kubernetes":{"enabled":{"locked":false,"value":false`,
		`"kubernetes":{"enabled":{"locked":false,"value":true`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))

	f.postSettings = nil
	err = f.d4m.SetSettingValue(ctx, "vm.kubernetes.enabled", "false")
	require.NoError(t, err)
	assert.Nil(t, f.postSettings)
}

func TestSetSettingValueFileSharing(t *testing.T) {
	f := newD4MFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	err := f.d4m.SetSettingValue(ctx, "vm.fileSharing", "/x,/y")
	require.NoError(t, err)

	expected := strings.Replace(postSettingsJSONV2,
		`"fileSharing":[{"cached":false,"path":"/home"}]`,
		`"fileSharing":[{"cached":false,"path":"/x"}, {"cached":false,"path":"/y"}]`, 1)
	assert.Equal(t,
		f.postSettings,
		f.readerToMap(strings.NewReader(expected)))

}

func TestChooseWorstError(t *testing.T) {
	tt := []struct {
		expected string
		errors   []error
	}{
		{
			"connection error",
			[]error{
				errors.Wrap(errors.New("connection error"), ""),
				withStatusCode{errors.New("404 error"), 404},
			},
		},
		{
			"500 error",
			[]error{
				withStatusCode{errors.New("500 error"), 500},
				withStatusCode{errors.New("404 error"), 404},
			},
		},
		{
			"first error",
			[]error{
				errors.Wrap(errors.New("first error"), ""),
				errors.Wrap(errors.New("second error"), ""),
			},
		},
	}

	for i, tc := range tt {
		t.Run(strconv.Itoa(i)+" "+tc.expected, func(t *testing.T) {
			err := chooseWorstError(tc.errors)
			assert.EqualError(t, errors.Cause(err), tc.expected)
		})
	}
}

// Pre DD 4.12
var getSettingsJSONV1 = `{"vm":{"proxy":{"exclude":{"value":"","locked":false},"http":{"value":"","locked":false},"https":{"value":"","locked":false},"mode":{"value":"system","locked":false}},"daemon":{"locks":[],"json":"{\"debug\":true,\"experimental\":false}"},"resources":{"cpus":{"value":2,"min":1,"locked":false,"max":8},"memoryMiB":{"value":8192,"min":1024,"locked":false,"max":16384},"diskSizeMiB":{"value":61035,"used":18486,"locked":false},"dataFolder":{"value":"\/Users\/nick\/Library\/Containers\/com.docker.docker\/Data\/vms\/0\/data","locked":false},"swapMiB":{"value":1024,"min":512,"locked":false,"max":4096}},"fileSharing":{"value":[{"path":"\/Users","cached":false},{"path":"\/Volumes","cached":false},{"path":"\/private","cached":false},{"path":"\/tmp","cached":false}],"locked":false},"kubernetes":{"enabled":{"value":false,"locked":false},"stackOrchestrator":{"value":false,"locked":false},"showSystemContainers":{"value":false,"locked":false}},"network":{"dns":{"locked":false},"vpnkitCIDR":{"value":"192.168.65.0\/24","locked":false},"automaticDNS":{"value":true,"locked":false}}},"desktop":{"exportInsecureDaemon":{"value":false,"locked":false},"useGrpcfuse":{"value":true,"locked":false},"backupData":{"value":false,"locked":false},"checkForUpdates":{"value":true,"locked":false},"useCredentialHelper":{"value":true,"locked":false},"autoStart":{"value":false,"locked":false},"analyticsEnabled":{"value":true,"locked":false}},"wslIntegration":{"distros":{"value":[],"locked":false},"enableIntegrationWithDefaultWslDistro":{"value":false,"locked":false}},"cli":{"useCloudCli":{"value":true,"locked":false},"experimental":{"value":true,"locked":false}}}`

// Post DD 4.12
var getSettingsJSONV2 = `{"vm":{"proxy":{"exclude":{"value":"","locked":false},"http":{"value":"","locked":false},"https":{"value":"","locked":false},"mode":{"value":"system","locked":false}},"daemon":{"value":"{\"builder\":{\"gc\":{\"defaultKeepStorage\":\"20GB\",\"enabled\":true}},\"experimental\":false,\"features\":{\"buildkit\":true}}","locked":false},"resources":{"cpus":{"value":2,"min":1,"max":8},"memoryMiB":{"value":5120,"min":1024,"max":15627},"diskSizeMiB":{"value":65536},"dataFolder":"/home/nick/.docker/desktop/vms/0/data","swapMiB":{"value":1536,"min":512,"max":4096}},"fileSharing":[{"path":"/home","cached":false}],"kubernetes":{"enabled":{"value":false,"locked":false},"showSystemContainers":{"value":false,"locked":false},"installed":true},"network":{"automaticDNS":false,"DNS":"","socksProxyPort":0,"vpnkitCIDR":{"value":"192.168.65.0/24","locked":false}}},"desktop":{"autoStart":false,"tipLastId":30,"exportInsecureDaemon":{"value":false,"locked":false},"disableTips":true,"analyticsEnabled":{"value":true,"locked":false},"enhancedContainerIsolation":{"value":false,"locked":false},"backupData":false,"tipLastViewedTime":1667005050000,"useVirtualizationFrameworkVirtioFS":true,"useVirtualizationFramework":false,"canUseVirtualizationFrameworkVirtioFS":false,"canUseVirtualizationFramework":false,"mustDisplayVirtualizationFrameworkSwitch":false,"disableHardwareAcceleration":false,"disableUpdate":{"value":false,"locked":false},"autoDownloadUpdates":{"value":false,"locked":false},"useNightlyBuildUpdates":{"value":false,"locked":false},"useVpnkit":true,"openUIOnStartupDisabled":false,"updateAvailableTime":0,"updateInstallTime":0,"useCredentialHelper":true,"displayedTutorial":true,"themeSource":"system","containerTerminal":"integrated","useContainerdSnapshotter":false,"allowExperimentalFeatures":true,"enableSegmentDebug":false,"wslEngineEnabled":{"value":false,"locked":false},"wslEnableGrpcfuse":false,"wslPreconditionMessage":"","noWindowsContainers":false,"useBackgroundIndexing":true},"cli":{"useComposeV2":false,"useGrpcfuse":true},"vpnkit":{"maxConnections":0,"maxPortIdleTime":300,"MTU":0,"allowedBindAddresses":"","transparentProxy":false},"extensions":{"enabled":true,"onlyMarketplaceExtensions":false,"showSystemContainers":true},"wslIntegration":{"distros":[],"enableIntegrationWithDefaultWslDistro":false}}`

// Pre DD 4.12
var postSettingsJSONV1 = `{"desktop":{"exportInsecureDaemon":false,"useGrpcfuse":true,"backupData":false,"checkForUpdates":true,"useCredentialHelper":true,"autoStart":false,"analyticsEnabled":true},"cli":{"useCloudCli":true,"experimental":true},"vm":{"daemon":"{\"debug\":true,\"experimental\":false}","fileSharing":[{"path":"/Users","cached":false},{"path":"/Volumes","cached":false},{"path":"/private","cached":false},{"path":"/tmp","cached":false}],"kubernetes":{"enabled":false,"stackOrchestrator":false,"showSystemContainers":false},"network":{"vpnkitCIDR":"192.168.65.0/24","automaticDNS":true},"proxy":{"exclude":"","http":"","https":"","mode":"system"},"resources":{"cpus":2,"memoryMiB":8192,"diskSizeMiB":61035,"dataFolder":"/Users/nick/Library/Containers/com.docker.docker/Data/vms/0/data","swapMiB":1024}},"wslIntegration":{"distros":[],"enableIntegrationWithDefaultWslDistro":false}}`

// Post DD 4.12
var postSettingsJSONV2 = `{"cli":{"useComposeV2":false,"useGrpcfuse":true},"desktop":{"allowExperimentalFeatures":true,"analyticsEnabled":{"locked":false,"value":true},"autoDownloadUpdates":{"locked":false,"value":false},"autoStart":false,"backupData":false,"canUseVirtualizationFramework":false,"canUseVirtualizationFrameworkVirtioFS":false,"containerTerminal":"integrated","disableHardwareAcceleration":false,"disableTips":true,"disableUpdate":{"locked":false,"value":false},"displayedTutorial":true,"enableSegmentDebug":false,"enhancedContainerIsolation":{"locked":false,"value":false},"exportInsecureDaemon":{"locked":false,"value":false},"mustDisplayVirtualizationFrameworkSwitch":false,"noWindowsContainers":false,"openUIOnStartupDisabled":false,"themeSource":"system","tipLastId":30,"tipLastViewedTime":1667005050000,"updateAvailableTime":0,"updateInstallTime":0,"useBackgroundIndexing":true,"useContainerdSnapshotter":false,"useCredentialHelper":true,"useNightlyBuildUpdates":{"locked":false,"value":false},"useVirtualizationFramework":false,"useVirtualizationFrameworkVirtioFS":true,"useVpnkit":true,"wslEnableGrpcfuse":false,"wslEngineEnabled":{"locked":false,"value":false},"wslPreconditionMessage":""},"extensions":{"enabled":true,"onlyMarketplaceExtensions":false,"showSystemContainers":true},"vm":{"daemon":{"locked":false,"value":"{\"builder\":{\"gc\":{\"defaultKeepStorage\":\"20GB\",\"enabled\":true}},\"experimental\":false,\"features\":{\"buildkit\":true}}"},"fileSharing":[{"cached":false,"path":"/home"}],"kubernetes":{"enabled":{"locked":false,"value":false},"installed":true,"showSystemContainers":{"locked":false,"value":false}},"network":{"DNS":"","automaticDNS":false,"socksProxyPort":0,"vpnkitCIDR":{"locked":false,"value":"192.168.65.0/24"}},"proxy":{"exclude":{"locked":false,"value":""},"http":{"locked":false,"value":""},"https":{"locked":false,"value":""},"mode":{"locked":false,"value":"system"}},"resources":{"cpus":{"max":8,"min":1,"value":2},"dataFolder":"/home/nick/.docker/desktop/vms/0/data","diskSizeMiB":{"value":65536},"memoryMiB":{"max":15627,"min":1024,"value":5120},"swapMiB":{"max":4096,"min":512,"value":1536}}},"vpnkit":{"MTU":0,"allowedBindAddresses":"","maxConnections":0,"maxPortIdleTime":300,"transparentProxy":false},"wslIntegration":{"distros":[],"enableIntegrationWithDefaultWslDistro":false}}`

type d4mFixture struct {
	t            *testing.T
	d4m          *DockerDesktopClient
	postSettings map[string]interface{}
	v            ddProtocol
}

func newD4MFixture(t *testing.T) *d4mFixture {
	f := &d4mFixture{t: t}
	f.v = ddProtocolV2
	f.d4m = &DockerDesktopClient{backendNativeClient: f, backendClient: f}
	return f
}

func (f *d4mFixture) readerToMap(r io.Reader) map[string]interface{} {
	result := make(map[string]interface{})
	err := json.NewDecoder(r).Decode(&result)
	require.NoError(f.t, err)
	return result
}

func (f *d4mFixture) Do(r *http.Request) (*http.Response, error) {
	settings := getSettingsJSONV2
	if f.v == ddProtocolV2 {
		require.Equal(f.t, r.URL.Path, "/app/settings")
	} else {
		if r.URL.Path == "/app/settings" {
			// Simulate an error so that we try the old endpoint.
			return nil, fmt.Errorf("Mock using V1, /app/settings endpoint doesn't exist")
		}
		require.Equal(f.t, r.URL.Path, "/settings")
		settings = getSettingsJSONV1
	}
	if r.Method == "POST" {
		f.postSettings = f.readerToMap(r.Body)

		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       closeReader{strings.NewReader("")},
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       closeReader{strings.NewReader(settings)},
	}, nil
}

func (f *d4mFixture) TearDown() {
}

type closeReader struct {
	io.Reader
}

func (c closeReader) Close() error { return nil }
