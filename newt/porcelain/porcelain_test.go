/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package porcelain

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func testInfo() *Info {
	return &Info{
		FormatVersion: FormatVersion,
		NewtVersion:   "1.16.0",
		Project:       Project{Name: "proj", Path: "/home/user/proj"},
		Repos: []Repo{
			{
				Name:      "apache-mynewt-core",
				Path:      "repos/apache-mynewt-core",
				Url:       "https://github.com/apache/mynewt-core.git",
				Requested: "1.12.0",
				Installed: "1.12.0",
				Commit:    "c81875e2",
			},
			{Name: "proj", Path: ".", Local: true},
		},
		Targets: []Target{
			{
				Name: "targets/blinky",
				Repo: "proj",
				Path: "targets/blinky",
				Vars: map[string]string{
					"bsp":    "@apache-mynewt-core/hw/bsp/nordic_pca10056",
					"app":    "@apache-mynewt-core/apps/blinky",
					"cflags": "-DFOO -DBAR",
				},
				Syscfg: []Setting{
					{
						Name:        "CONSOLE_UART_BAUD",
						Value:       "1000000",
						Default:     "115200",
						Description: "Console UART baud rate.",
						Type:        "raw",
						State:       "good",
						Package:     "@apache-mynewt-core/sys/console",
						Choices:     []string{},
						Restrictions: []Restriction{
							{Type: "notnull"},
							{Type: "expr", Expr: "CONSOLE_UART"},
						},
						History: []SettingPoint{
							{"@apache-mynewt-core/sys/console", "115200"},
							{"targets/blinky", "1000000"},
						},
					},
					{
						Name:         "LOG_LEVEL",
						Value:        "",
						Package:      "newt",
						Choices:      []string{"0", "1"},
						Restrictions: []Restriction{},
						History:      []SettingPoint{},
					},
				},
				SyscfgErrors: []string{"Setting history (newest -> oldest):"},
			},
		},
		Packages: []Package{
			{
				Name:        "@apache-mynewt-core/apps/blinky",
				Type:        "app",
				Repo:        "apache-mynewt-core",
				Path:        "repos/apache-mynewt-core/apps/blinky",
				Description: "Blinks a LED.\nTwice \\ a second.",
			},
		},
	}
}

func TestWritePorcelain(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePorcelain(&buf, testInfo()); err != nil {
		t.Fatal(err)
	}

	want := `project proj format_version 1
project proj newt_version 1.16.0
project proj path /home/user/proj
repo apache-mynewt-core path repos/apache-mynewt-core
repo apache-mynewt-core local false
repo apache-mynewt-core url https://github.com/apache/mynewt-core.git
repo apache-mynewt-core requested 1.12.0
repo apache-mynewt-core installed 1.12.0
repo apache-mynewt-core commit c81875e2
repo apache-mynewt-core needs_upgrade false
repo proj path .
repo proj local true
repo proj needs_upgrade false
target targets/blinky repo proj
target targets/blinky path targets/blinky
target targets/blinky var.app @apache-mynewt-core/apps/blinky
target targets/blinky var.bsp @apache-mynewt-core/hw/bsp/nordic_pca10056
target targets/blinky var.cflags -DFOO -DBAR
target targets/blinky syscfg_error Setting history (newest -> oldest):
target targets/blinky syscfg.CONSOLE_UART_BAUD.value 1000000
target targets/blinky syscfg.CONSOLE_UART_BAUD.default 115200
target targets/blinky syscfg.CONSOLE_UART_BAUD.description Console UART baud rate.
target targets/blinky syscfg.CONSOLE_UART_BAUD.type raw
target targets/blinky syscfg.CONSOLE_UART_BAUD.state good
target targets/blinky syscfg.CONSOLE_UART_BAUD.package @apache-mynewt-core/sys/console
target targets/blinky syscfg.CONSOLE_UART_BAUD.restriction notnull
target targets/blinky syscfg.CONSOLE_UART_BAUD.restriction expr CONSOLE_UART
target targets/blinky syscfg.CONSOLE_UART_BAUD.history @apache-mynewt-core/sys/console 115200
target targets/blinky syscfg.CONSOLE_UART_BAUD.history targets/blinky 1000000
target targets/blinky syscfg.LOG_LEVEL.package newt
target targets/blinky syscfg.LOG_LEVEL.choices 0 1
package @apache-mynewt-core/apps/blinky type app
package @apache-mynewt-core/apps/blinky repo apache-mynewt-core
package @apache-mynewt-core/apps/blinky path repos/apache-mynewt-core/apps/blinky
package @apache-mynewt-core/apps/blinky description Blinks a LED.\nTwice \\ a second.
`
	if got := buf.String(); got != want {
		t.Errorf("unexpected output:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, testInfo()); err != nil {
		t.Fatal(err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	// The keys are part of the format; renaming any of them breaks readers.
	keys := map[string][]string{
		"":         {"format_version", "newt_version", "packages", "project", "repos", "targets"},
		"project":  {"name", "path"},
		"repos":    {"commit", "dirty", "error", "installed", "local", "name", "needs_upgrade", "path", "requested", "url"},
		"targets":  {"name", "path", "repo", "syscfg", "syscfg_errors", "vars"},
		"packages": {"description", "name", "path", "repo", "type"},
	}
	for field, want := range keys {
		obj := got
		if field != "" {
			switch v := got[field].(type) {
			case []interface{}:
				obj = v[0].(map[string]interface{})
			default:
				obj = v.(map[string]interface{})
			}
		}
		if have := sortedKeys(stringKeys(obj)); !reflect.DeepEqual(have, want) {
			t.Errorf("keys of %q: got %v, want %v", field, have, want)
		}
	}

	setting := got["targets"].([]interface{})[0].(map[string]interface{})["syscfg"].([]interface{})[0].(map[string]interface{})
	wantSetting := []string{"choices", "default", "description", "history", "name", "package", "restrictions", "state", "type", "value", "value_ref"}
	if have := sortedKeys(stringKeys(setting)); !reflect.DeepEqual(have, wantSetting) {
		t.Errorf("keys of syscfg settings: got %v, want %v", have, wantSetting)
	}

	var info Info
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&info, testInfo()) {
		t.Errorf("JSON does not round-trip: %+v", info)
	}
}

func stringKeys(m map[string]interface{}) map[string]string {
	s := make(map[string]string, len(m))
	for k := range m {
		s[k] = ""
	}
	return s
}
