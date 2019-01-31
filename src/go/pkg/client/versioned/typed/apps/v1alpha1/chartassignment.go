// Copyright 2019 The Google Cloud Robotics Authors
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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"time"

	v1alpha1 "cloud-robotics.googlesource.com/cloud-robotics/pkg/apis/apps/v1alpha1"
	scheme "cloud-robotics.googlesource.com/cloud-robotics/pkg/client/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ChartAssignmentsGetter has a method to return a ChartAssignmentInterface.
// A group's client should implement this interface.
type ChartAssignmentsGetter interface {
	ChartAssignments() ChartAssignmentInterface
}

// ChartAssignmentInterface has methods to work with ChartAssignment resources.
type ChartAssignmentInterface interface {
	Create(*v1alpha1.ChartAssignment) (*v1alpha1.ChartAssignment, error)
	Update(*v1alpha1.ChartAssignment) (*v1alpha1.ChartAssignment, error)
	UpdateStatus(*v1alpha1.ChartAssignment) (*v1alpha1.ChartAssignment, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.ChartAssignment, error)
	List(opts v1.ListOptions) (*v1alpha1.ChartAssignmentList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ChartAssignment, err error)
	ChartAssignmentExpansion
}

// chartAssignments implements ChartAssignmentInterface
type chartAssignments struct {
	client rest.Interface
}

// newChartAssignments returns a ChartAssignments
func newChartAssignments(c *AppsV1alpha1Client) *chartAssignments {
	return &chartAssignments{
		client: c.RESTClient(),
	}
}

// Get takes name of the chartAssignment, and returns the corresponding chartAssignment object, and an error if there is any.
func (c *chartAssignments) Get(name string, options v1.GetOptions) (result *v1alpha1.ChartAssignment, err error) {
	result = &v1alpha1.ChartAssignment{}
	err = c.client.Get().
		Resource("chartassignments").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ChartAssignments that match those selectors.
func (c *chartAssignments) List(opts v1.ListOptions) (result *v1alpha1.ChartAssignmentList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ChartAssignmentList{}
	err = c.client.Get().
		Resource("chartassignments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested chartAssignments.
func (c *chartAssignments) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("chartassignments").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a chartAssignment and creates it.  Returns the server's representation of the chartAssignment, and an error, if there is any.
func (c *chartAssignments) Create(chartAssignment *v1alpha1.ChartAssignment) (result *v1alpha1.ChartAssignment, err error) {
	result = &v1alpha1.ChartAssignment{}
	err = c.client.Post().
		Resource("chartassignments").
		Body(chartAssignment).
		Do().
		Into(result)
	return
}

// Update takes the representation of a chartAssignment and updates it. Returns the server's representation of the chartAssignment, and an error, if there is any.
func (c *chartAssignments) Update(chartAssignment *v1alpha1.ChartAssignment) (result *v1alpha1.ChartAssignment, err error) {
	result = &v1alpha1.ChartAssignment{}
	err = c.client.Put().
		Resource("chartassignments").
		Name(chartAssignment.Name).
		Body(chartAssignment).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *chartAssignments) UpdateStatus(chartAssignment *v1alpha1.ChartAssignment) (result *v1alpha1.ChartAssignment, err error) {
	result = &v1alpha1.ChartAssignment{}
	err = c.client.Put().
		Resource("chartassignments").
		Name(chartAssignment.Name).
		SubResource("status").
		Body(chartAssignment).
		Do().
		Into(result)
	return
}

// Delete takes name of the chartAssignment and deletes it. Returns an error if one occurs.
func (c *chartAssignments) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("chartassignments").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *chartAssignments) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("chartassignments").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched chartAssignment.
func (c *chartAssignments) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ChartAssignment, err error) {
	result = &v1alpha1.ChartAssignment{}
	err = c.client.Patch(pt).
		Resource("chartassignments").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
