// Copyright 2016 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration_test

import (
	"testing"

	"github.com/nmiyake/pkg/gofiles"
	"github.com/palantir/godel/v2/framework/pluginapitester"
	"github.com/palantir/godel/v2/pkg/products"
	"github.com/palantir/okgo/okgotester"
	"github.com/stretchr/testify/require"
)

const (
	okgoPluginLocator  = "com.palantir.okgo:check-plugin:1.0.0"
	okgoPluginResolver = "https://palantir.bintray.com/releases/{{GroupPath}}/{{Product}}/{{Version}}/{{Product}}-{{Version}}-{{OS}}-{{Arch}}.tgz"
)

func TestCheck(t *testing.T) {
	const godelYML = `exclude:
  names:
    - "\\..+"
    - "vendor"
  paths:
    - "godel"
`

	assetPath, err := products.Bin("deadcode-asset")
	require.NoError(t, err)

	configFiles := map[string]string{
		"godel/config/godel.yml":        godelYML,
		"godel/config/check-plugin.yml": "",
	}

	pluginProvider, err := pluginapitester.NewPluginProviderFromLocator(okgoPluginLocator, okgoPluginResolver)
	require.NoError(t, err)

	okgotester.RunAssetCheckTest(t,
		pluginProvider,
		pluginapitester.NewAssetProvider(assetPath),
		"deadcode",
		"",
		[]okgotester.AssetTestCase{
			{
				Name: "deadcode in file",
				Specs: []gofiles.GoFileSpec{
					{
						RelPath: "go.mod",
						Src:     `module foo`,
					},
					{
						RelPath: "foo.go",
						Src:     `package main; func main() {}; var unused int`,
					},
				},
				ConfigFiles: configFiles,
				WantError:   true,
				WantOutput: `Running deadcode...
foo.go:1:35: unused is unused
Finished deadcode
Check(s) produced output: [deadcode]
`,
			},
			{
				Name: "deadcode in file from inner directory",
				Specs: []gofiles.GoFileSpec{
					{
						RelPath: "go.mod",
						Src:     `module foo`,
					},
					{
						RelPath: "foo.go",
						Src:     `package main; func main() {}; var unused int`,
					},
					{
						RelPath: "inner/bar",
					},
				},
				ConfigFiles: configFiles,
				Wd:          "inner",
				WantError:   true,
				WantOutput: `Running deadcode...
../foo.go:1:35: unused is unused
Finished deadcode
Check(s) produced output: [deadcode]
`,
			},
		},
	)
}

func TestUpgradeConfig(t *testing.T) {
	pluginProvider, err := pluginapitester.NewPluginProviderFromLocator(okgoPluginLocator, okgoPluginResolver)
	require.NoError(t, err)

	assetPath, err := products.Bin("deadcode-asset")
	require.NoError(t, err)
	assetProvider := pluginapitester.NewAssetProvider(assetPath)

	pluginapitester.RunUpgradeConfigTest(t,
		pluginProvider,
		[]pluginapitester.AssetProvider{assetProvider},
		[]pluginapitester.UpgradeConfigTestCase{
			{
				Name: `legacy configuration with empty "args" field is updated`,
				ConfigFiles: map[string]string{
					"godel/config/check.yml": `
checks:
  deadcode:
    filters:
      - value: "should have comment or be unexported"
      - type: name
        value: ".*.pb.go"
`,
				},
				Legacy: true,
				WantOutput: `Upgraded configuration for check-plugin.yml
`,
				WantFiles: map[string]string{
					"godel/config/check-plugin.yml": `checks:
  deadcode:
    filters:
    - value: should have comment or be unexported
    exclude:
      names:
      - .*.pb.go
`,
				},
			},
			{
				Name: `legacy configuration with non-empty "args" field fails`,
				ConfigFiles: map[string]string{
					"godel/config/check.yml": `
checks:
  deadcode:
    args:
      - "-foo"
`,
				},
				Legacy:    true,
				WantError: true,
				WantOutput: `Failed to upgrade configuration:
	godel/config/check-plugin.yml: failed to upgrade configuration: failed to upgrade check "deadcode" legacy configuration: failed to upgrade asset configuration: deadcode-asset does not support legacy configuration with a non-empty "args" field
`,
				WantFiles: map[string]string{
					"godel/config/check.yml": `
checks:
  deadcode:
    args:
      - "-foo"
`,
				},
			},
			{
				Name: `empty v0 config works`,
				ConfigFiles: map[string]string{
					"godel/config/check-plugin.yml": `
checks:
  deadcode:
    skip: true
    # comment preserved
    config:
`,
				},
				WantOutput: ``,
				WantFiles: map[string]string{
					"godel/config/check-plugin.yml": `
checks:
  deadcode:
    skip: true
    # comment preserved
    config:
`,
				},
			},
			{
				Name: `non-empty v0 config does not work`,
				ConfigFiles: map[string]string{
					"godel/config/check-plugin.yml": `
checks:
  deadcode:
    config:
      # comment
      key: value
`,
				},
				WantError: true,
				WantOutput: `Failed to upgrade configuration:
	godel/config/check-plugin.yml: failed to upgrade check "deadcode" configuration: failed to upgrade asset configuration: deadcode-asset does not currently support configuration
`,
				WantFiles: map[string]string{
					"godel/config/check-plugin.yml": `
checks:
  deadcode:
    config:
      # comment
      key: value
`,
				},
			},
		},
	)
}
