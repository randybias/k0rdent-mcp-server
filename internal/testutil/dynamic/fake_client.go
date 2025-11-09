package dynamic

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

// FakeDynamicClient provides a minimal dynamic.Interface implementation for unit tests.
type FakeDynamicClient struct {
	objects map[fakeResourceKey]*unstructured.Unstructured
}

type fakeResourceKey struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
}

// NewFakeDynamicClient creates an empty fake client.
func NewFakeDynamicClient() *FakeDynamicClient {
	return &FakeDynamicClient{
		objects: make(map[fakeResourceKey]*unstructured.Unstructured),
	}
}

// Add stores the provided objects under the supplied GVR.
func (c *FakeDynamicClient) Add(gvr schema.GroupVersionResource, objs ...*unstructured.Unstructured) {
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		key := fakeResourceKey{
			gvr:       gvr,
			namespace: obj.GetNamespace(),
			name:      obj.GetName(),
		}
		c.objects[key] = cloneUnstructured(obj)
	}
}

// GetObject returns a stored object copy for verification.
func (c *FakeDynamicClient) GetObject(gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, bool) {
	key := fakeResourceKey{gvr: gvr, namespace: namespace, name: name}
	obj, ok := c.objects[key]
	if !ok {
		return nil, false
	}
	return cloneUnstructured(obj), true
}

// Resource implements dynamic.Interface.
func (c *FakeDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &fakeNamespaceableResource{client: c, gvr: gvr}
}

type fakeNamespaceableResource struct {
	client *FakeDynamicClient
	gvr    schema.GroupVersionResource
}

func (r *fakeNamespaceableResource) Namespace(namespace string) dynamic.ResourceInterface {
	return &fakeResourceInterface{
		client:    r.client,
		gvr:       r.gvr,
		namespace: namespace,
	}
}

func (r *fakeNamespaceableResource) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.Namespace("").Create(ctx, obj, opts, subresources...)
}

func (r *fakeNamespaceableResource) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.Namespace("").Update(ctx, obj, opts, subresources...)
}

func (r *fakeNamespaceableResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return r.Namespace("").UpdateStatus(ctx, obj, opts)
}

func (r *fakeNamespaceableResource) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	return r.Namespace("").Delete(ctx, name, opts, subresources...)
}

func (r *fakeNamespaceableResource) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return r.Namespace("").DeleteCollection(ctx, opts, listOpts)
}

func (r *fakeNamespaceableResource) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.Namespace("").Get(ctx, name, opts, subresources...)
}

func (r *fakeNamespaceableResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return r.Namespace("").List(ctx, opts)
}

func (r *fakeNamespaceableResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return r.Namespace("").Watch(ctx, opts)
}

func (r *fakeNamespaceableResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.Namespace("").Patch(ctx, name, pt, data, opts, subresources...)
}

func (r *fakeNamespaceableResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return r.Namespace("").Apply(ctx, name, obj, opts, subresources...)
}

func (r *fakeNamespaceableResource) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return r.Namespace("").ApplyStatus(ctx, name, obj, opts)
}

type fakeResourceInterface struct {
	client    *FakeDynamicClient
	gvr       schema.GroupVersionResource
	namespace string
}

func (r *fakeResourceInterface) key(name string) fakeResourceKey {
	return fakeResourceKey{gvr: r.gvr, namespace: r.namespace, name: name}
}

func (r *fakeResourceInterface) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("create not implemented in fake client")
}

func (r *fakeResourceInterface) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("update not implemented in fake client")
}

func (r *fakeResourceInterface) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("update status not implemented in fake client")
}

func (r *fakeResourceInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	return fmt.Errorf("delete not implemented in fake client")
}

func (r *fakeResourceInterface) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return fmt.Errorf("delete collection not implemented in fake client")
}

func (r *fakeResourceInterface) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	obj, ok := r.client.objects[r.key(name)]
	if !ok {
		return nil, apierrors.NewNotFound(r.gvr.GroupResource(), name)
	}
	return cloneUnstructured(obj), nil
}

func (r *fakeResourceInterface) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("list not implemented in fake client")
}

func (r *fakeResourceInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("watch not implemented in fake client")
}

func (r *fakeResourceInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("patch not implemented in fake client")
}

func (r *fakeResourceInterface) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	current, ok := r.client.objects[r.key(name)]
	if !ok {
		return nil, apierrors.NewNotFound(r.gvr.GroupResource(), name)
	}
	updated := cloneUnstructured(current)
	if specObj, ok := obj.Object["spec"].(map[string]any); ok {
		if serviceSpec, ok := specObj["serviceSpec"].(map[string]any); ok {
			mergeServiceSpec(updated, serviceSpec)
		}
	}
	result := cloneUnstructured(updated)
	if len(opts.DryRun) == 0 {
		r.client.objects[r.key(name)] = cloneUnstructured(updated)
	}
	return result, nil
}

func (r *fakeResourceInterface) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return r.Apply(ctx, name, obj, opts)
}

func mergeServiceSpec(dest *unstructured.Unstructured, incoming map[string]any) {
	if len(incoming) == 0 {
		return
	}
	specObj, _ := dest.Object["spec"].(map[string]any)
	if specObj == nil {
		specObj = map[string]any{}
		dest.Object["spec"] = specObj
	}
	existing, _ := specObj["serviceSpec"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for k, v := range incoming {
		existing[k] = v
	}
	specObj["serviceSpec"] = existing
}

func cloneUnstructured(obj *unstructured.Unstructured) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	copy := &unstructured.Unstructured{
		Object: cloneMap(obj.Object),
	}
	return copy
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneSlice(in []any) []any {
	if in == nil {
		return nil
	}
	out := make([]any, len(in))
	for i := range in {
		out[i] = cloneValue(in[i])
	}
	return out
}

func cloneValue(val any) any {
	switch v := val.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		return cloneSlice(v)
	default:
		return v
	}
}
