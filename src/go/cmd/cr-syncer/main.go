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

// The CR syncer syncs custom resources between a remote Kubernetes cluster
// and the local Kubernetes cluster. The spec part is copied from upstream to
// downstream, and the status part is copied from downstream to upstream.
//
// The behaviour can be customized by annotations on the CRDs.
// cr-syncer.cloudrobotics.com/filter-by-robot-name: <bool>
//   If true, only sync CRs that have a label 'cloudrobotics.com/robot-name: <robot-name>'
//   that matches the robot-name arg given on the command line.
// cr-syncer.cloudrobotics.com/status-subtree: <string>
//   If specified, only sync the given subtree of the Status field. This is useful
//   if resources have a shared status.
// cr-syncer.cloudrobotics.com/spec-source: <string>
//   If unset or "cloud", the source of truth for object existence and specs (upstream) is
//   the remote cluster and for status it's local (downstream). If set to "robot", the roles
//   are reversed.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/motemen/go-loghttp"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	crdtypes "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	crdclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	crdinformer "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

const (
	// Resync informers every 5 minutes. This will cause all current resources
	// to be sent as updates once again, which will trigger reconciliation on those
	// objects and thus fix any potential drift.
	resyncPeriod = 5 * time.Minute
)

var (
	remoteServer = flag.String("remote-server", "", "Remote Kubernetes server")
	robotName    = flag.String("robot-name", "", "Robot we are running on, can be used for selective syncing")
	verbose      = flag.Bool("verbose", false, "Enable verbose logging")
)

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

// restConfigForRemote assembles the K8s REST config for the remote server.
func restConfigForRemote(ctx context.Context) (*rest.Config, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host:    *remoteServer,
		APIPath: "/apis",
		WrapTransport: func(base http.RoundTripper) http.RoundTripper {
			rt := &PrefixingRoundtripper{
				Prefix: "/apis/core.kubernetes",
				Base:   &oauth2.Transport{Source: tokenSource, Base: base},
			}
			if *verbose {
				return &loghttp.Transport{Transport: rt}
			}
			return rt
		},
	}, nil
}

// syncMetaAndSpec ensures that the downstream object matches the source in metadata and spec.
// It keeps the downstream status, if there is any.
// If the upstream object is pending deletion, the deletion is propagated downstream.
func syncMetaAndSpec(target dynamic.ResourceInterface, source *unstructured.Unstructured) error {
	o, err := target.Get(source.GetName(), metav1.GetOptions{})
	if err != nil {
		status, isStatus := err.(*errors.StatusError)
		if !isStatus || status.ErrStatus.Code != http.StatusNotFound {
			return fmt.Errorf("Get for current state failed: %v", err)
		}

		checkResourceVersionAnnotation(o, source)

		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(source.GroupVersionKind())
		o.SetName(source.GetName())
		// Don't copy output-only and generated fields.
		o.SetAnnotations(source.GetAnnotations())
		setAnnotation(o, annotationOwnedByUpstream, "true")
		o.SetLabels(source.GetLabels())
		o.Object["spec"] = source.Object["spec"]
		o.Object["status"] = source.Object["status"]

		_, err = target.Create(o, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("Create failed: %v", err)
		}
		return nil
	}

	// Don't copy output-only and generated fields.
	o.SetAnnotations(source.GetAnnotations())
	deleteAnnotation(o, annotationResourceVersion)
	setAnnotation(o, annotationOwnedByUpstream, "true")
	o.SetLabels(source.GetLabels())
	o.Object["spec"] = source.Object["spec"]

	// TODO(swolter): We could elide the Update if no updates happened. Checking
	// for equality is hard, though, and inBytes vs outBytes comparisons don't
	// work. DeepEqual might work.
	_, err = target.Update(o, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Update failed: %v", err)
	}
	// Propagate in-progress deletion downstream in case upstream final deletion
	// depends on downstream cleanup to complete.
	// Since the deletion timestamp field is immutable, we've to do this via
	// an explicit delete call.
	if source.GetDeletionTimestamp() == nil {
		return nil
	}
	if o.GetDeletionTimestamp() != nil {
		return nil // Already deleted.
	}
	if err := target.Delete(o.GetName(), nil); err != nil {
		status, ok := err.(*errors.StatusError)
		if ok && status.ErrStatus.Code == http.StatusNotFound {
			log.Printf("Resource already deleted: %s", o.GetName())
			return nil
		}
		return fmt.Errorf("delete failed: %s", err)
	}
	return nil
}

func setAnnotation(o *unstructured.Unstructured, key, value string) {
	annotations := o.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	o.SetAnnotations(annotations)
}

func deleteAnnotation(o *unstructured.Unstructured, key string) {
	annotations := o.GetAnnotations()
	if annotations != nil {
		delete(annotations, key)
	}
	if len(annotations) > 0 {
		o.SetAnnotations(annotations)
	} else {
		o.SetAnnotations(nil)
	}

}

// checkResourceVersionAnnotation checks if identical resource versions between the downstream CR and the
// "cr-syncer.cloudrobotics.com/remote-resource-version" annotation in the upstream CR also results in
// identical status fields. This is an attempt to detect when multiple CR-Syncer instances execute
// write operation to the same upstream CR.
func checkResourceVersionAnnotation(target, source *unstructured.Unstructured) {
	sourceRV := source.GetResourceVersion()
	if target == nil || source == nil {
		return
	}
	targetAnnotations := target.GetAnnotations()
	if targetAnnotations == nil {
		return
	}
	targetRV, ok := targetAnnotations[annotationResourceVersion]
	if !ok {
		return
	}
	if sourceRV == targetRV && !reflect.DeepEqual(target.Object["status"], source.Object["status"]) {
		log.Printf("The upstream status of %s %s doesn't match the downstream status. "+
			"Maybe it has been overwritten by something else.",
			sourceRV, targetRV)
	}
	return
}

// syncStatus updates the status and finalizers of upstream resources to match
// the downstream cluster.
// isSubresource indicates whether the status section is defined as a subresource
// in the CRD.
func syncStatus(target dynamic.ResourceInterface, subtree string, source *unstructured.Unstructured, isSubresource bool) (*unstructured.Unstructured, error) {
	// TODO(swolter): A local cache would avoid spurious loads of the remote resources.
	o, err := target.Get(source.GetName(), metav1.GetOptions{})
	if err != nil {
		status, isStatus := err.(*errors.StatusError)
		if isStatus && status.ErrStatus.Code == http.StatusNotFound {
			log.Printf("Upstream resource already deleted: %s", source.GetName())
			return nil, nil
		}
		return nil, fmt.Errorf("Loading failed: %v", err)
	}
	if subtree == "" {
		o.Object["status"] = source.Object["status"]
		// Synchronize finalizers so the spec source cluster can block deletion
		// until the controlling cluster has completed potential cleanup operations.
		// This is only done for CRs which don't have status subtrees. Those
		// have controllers in multiple clusters and finalizers cannot be reliably
		// synchronized between them.
		o.SetFinalizers(source.GetFinalizers())
	} else if source.Object["status"] != nil {
		sourceMap, ok := source.Object["status"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Expected status of %s in downstream cluster to be a dict", source.GetName())
		}
		if o.Object["status"] == nil {
			o.Object["status"] = make(map[string]interface{})
		}
		upstreamMap, ok := o.Object["status"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Expected status of %s in upstream cluster to be a dict", source.GetName())
		}
		if sourceMap[subtree] != nil {
			upstreamMap[subtree] = sourceMap[subtree]
		} else {
			delete(upstreamMap, subtree)
		}
	}
	setAnnotation(o, annotationResourceVersion, source.GetResourceVersion())

	// We need to make a dedicated UpdateStatus call if the status is defined
	// as an explicit subresource of the CRD.
	// We need a full update below in any case to propagate finalizer removal
	// and the revision annotation.
	// This reduces the atomicity of our status propagation by necessity. However,
	// not declaring status a subresource will disable generation tracking by the
	// API server, which we need to verify that a resource has been processed
	// by its controller.
	if isSubresource {
		// Status must not be null/nil.
		if o.Object["status"] == nil {
			o.Object["status"] = struct{}{}
		}
		if _, err := target.UpdateStatus(o, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("Status update failed: %v", err)
		}
	}
	// TODO(swolter): We could elide the Update if no updates happened. Checking
	// for equality is hard, though, and inBytes vs outBytes comparisons don't
	// work. DeepEqual might work.
	updated, err := target.Update(o, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("Update failed: %v", err)
	}
	return updated, nil
}

type CrdChange struct {
	Type watch.EventType
	CRD  *crdtypes.CustomResourceDefinition
}

func streamCrds(done <-chan struct{}, clientset crdclientset.Interface, crds chan<- CrdChange) error {
	factory := crdinformer.NewSharedInformerFactory(clientset, 0)
	informer := factory.Apiextensions().V1beta1().CustomResourceDefinitions().Informer()

	go informer.Run(done)

	log.Printf("Syncing cache for CRDs")
	ok := cache.WaitForCacheSync(done, informer.HasSynced)
	if !ok {
		return fmt.Errorf("WaitForCacheSync failed")
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			crds <- CrdChange{Type: watch.Added, CRD: obj.(*crdtypes.CustomResourceDefinition)}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			crds <- CrdChange{Type: watch.Modified, CRD: newObj.(*crdtypes.CustomResourceDefinition)}
		},
		DeleteFunc: func(obj interface{}) {
			crds <- CrdChange{Type: watch.Deleted, CRD: obj.(*crdtypes.CustomResourceDefinition)}
		},
	})
	return nil
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	ctx := context.Background()

	localConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	if *verbose {
		localConfig.WrapTransport = func(base http.RoundTripper) http.RoundTripper {
			return &loghttp.Transport{Transport: base}
		}
	}
	local, err := dynamic.NewForConfig(localConfig)
	if err != nil {
		log.Fatal(err)
	}
	remoteConfig, err := restConfigForRemote(ctx)
	if err != nil {
		log.Fatal(err)
	}
	remote, err := dynamic.NewForConfig(remoteConfig)
	if err != nil {
		log.Fatal(err)
	}
	crds := make(chan CrdChange)
	if err := streamCrds(ctx.Done(), crdclientset.NewForConfigOrDie(localConfig), crds); err != nil {
		log.Fatalf("Unable to stream CRDs from local Kubernetes: %v", err)
	}
	syncers := make(map[string]*crSyncer)
	for crd := range crds {
		name := crd.CRD.GetName()

		if cur, ok := syncers[name]; ok {
			if crd.Type == watch.Added {
				log.Printf("Warning: Already had a running sync for freshly added %s", name)
			}
			cur.stop()
			delete(syncers, name)
		}
		if crd.Type == watch.Added || crd.Type == watch.Modified {
			// The modify procedure is very heavyweight: We throw away
			// the informer for the CRD (read: all cached data) on every
			// modification and recreate it. If that ever turns out to
			// be a problem, we should use a shared informer cache
			// instead.
			s, err := newCRSyncer(*crd.CRD, local, remote, *robotName)
			if err != nil {
				log.Printf("skipping custom resource %s: %s", name, err)
				continue
			}
			syncers[name] = s
			go s.run()
		}
	}
}
