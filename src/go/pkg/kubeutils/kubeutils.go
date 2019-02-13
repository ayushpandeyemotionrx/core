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

package kubeutils

import (
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	LocalContext = "kubernetes-admin@kubernetes"

	localConfig            = "~/.kube/config"
	deletionTimeoutSeconds = 60
)

// Expand paths of the form "~/path" to absolute paths.
func ExpandUser(path string) string {
	if path[:2] != "~/" {
		return path
	}
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, path[2:])
}

// CloudKubernetesContextName generates the name of the cloud kubernetes context from the GCP
// project ID and region.
func CloudKubernetesContextName(projectID, region string) string {
	return fmt.Sprintf("gke_%s_%s-c_cloud-robotics", projectID, region)
}

// GetCloudKubernetesContext returns the name of the cloud kubernetes context.
func GetCloudKubernetesContext() (string, error) {
	gcpProjectID, defined := os.LookupEnv("GCP_PROJECT_ID")
	if !defined {
		return "", fmt.Errorf("GCP_PROJECT_ID environment variable is not defined")
	}
	gcpRegion, defined := os.LookupEnv("GCP_REGION")
	if !defined {
		return "", fmt.Errorf("GCP_REGION environment variable is not defined")
	}

	return CloudKubernetesContextName(gcpProjectID, gcpRegion), nil
}

// GetRobotKubernetesContext returns the name of the robot kubernetes context provided by the
// kubernetes-relay-client.
func GetRobotKubernetesContext() (string, error) {
	gcpProjectID, defined := os.LookupEnv("GCP_PROJECT_ID")
	if !defined {
		return "", fmt.Errorf("GCP_PROJECT_ID environment variable is not defined")
	}

	return fmt.Sprintf("%s-robot", gcpProjectID), nil
}

// LoadOutOfClusterConfig loads a local kubernetes config on the robot or workstation.
func LoadOutOfClusterConfigLocal() (*rest.Config, error) {
	return LoadOutOfClusterConfig(LocalContext)
}

func LoadOutOfClusterConfig(context string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = ExpandUser(localConfig)
	overrides := &clientcmd.ConfigOverrides{}
	overrides.CurrentContext = context
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return cfg.ClientConfig()
}

// PrefixingRoundtripper is a HTTP roundtripper that adds a specified prefix to
// all HTTP requests. We need to use it instead of setting APIPath because
// autogenerated and dynamic Kubernetes clients overwrite the REST config's
// APIPath.
type PrefixingRoundtripper struct {
	Prefix string
	Base   http.RoundTripper
}

func (pr *PrefixingRoundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	// Avoid an extra roundtrip for the protocol upgrade
	r.URL.Scheme = "https"
	if !strings.HasPrefix(r.URL.Path, pr.Prefix+"/") {
		r.URL.Path = pr.Prefix + r.URL.Path
	}
	resp, err := pr.Base.RoundTrip(r)
	return resp, err
}

// BuildCloudKubernetesConfig build a kubernetes config for authenticated access to the cloud
// project.
func BuildCloudKubernetesConfig(ts oauth2.TokenSource, remoteServer string) *rest.Config {
	return &rest.Config{
		Host:    remoteServer,
		APIPath: "/apis",
		WrapTransport: func(base http.RoundTripper) http.RoundTripper {
			rt := &PrefixingRoundtripper{
				Prefix: "/apis/core.kubernetes",
				Base:   &oauth2.Transport{Source: ts, Base: base},
			}
			return rt
		},
	}
}

// UpdateSecret (over-) writes a k8s secret.
func UpdateSecret(k8s *kubernetes.Clientset, name string, secretType corev1.SecretType, data map[string][]byte) error {
	s := k8s.CoreV1().Secrets(corev1.NamespaceDefault)
	if err := s.Delete(name, nil); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	_, err := s.Create(&corev1.Secret{
		Type: secretType,
		Data: data,
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	return err
}
