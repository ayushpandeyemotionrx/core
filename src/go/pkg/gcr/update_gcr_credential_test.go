// Copyright 2019 The Cloud Robotics Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcr

import (
	"bytes"
	"testing"
)

func TestDockercfgJSON(t *testing.T) {
	expectedJSON := []byte(`{"https://eu.gcr.io":{"username":"oauth2accesstoken","password":"ya29.yaddayadda","email":"not@val.id","auth":"b2F1dGgyYWNjZXNzdG9rZW46eWEyOS55YWRkYXlhZGRh"}}`)
	gotJSON := dockercfgJSON("ya29.yaddayadda")

	if !bytes.Equal(expectedJSON, gotJSON) {
		t.Errorf("Expected:\n  %s\nGot:\n  %s\n", expectedJSON, gotJSON)
	}
}
