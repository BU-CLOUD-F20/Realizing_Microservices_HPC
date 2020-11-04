package podset

import (
	"context"
	"reflect"
        "strings"
        "fmt"
	"strconv"
		
	appv1alpha1 "podset-operator/pkg/apis/app/v1alpha1"
	
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
        k8sv1 "k8s.io/api/core/v1"
        "k8s.io/apimachinery/pkg/api/resource"
        "github.com/spf13/pflag"
        k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "kubevirt.io/client-go/kubecli"
        kubevirtv1 "kubevirt.io/client-go/api/v1"

)

var log = logf.Log.WithName("controller_podset")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PodSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePodSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("podset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PodSet
	err = c.Watch(&source.Kind{Type: &appv1alpha1.PodSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner PodSet
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.PodSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePodSet{}

// ReconcilePodSet reconciles a PodSet object
type ReconcilePodSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PodSet object and makes changes based on the state read
// and what is in the PodSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
var vmCount int = 1

func (r *ReconcilePodSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PodSet")

	// Fetch the PodSet instance
	podSet := &appv1alpha1.PodSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, podSet)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// List all pods owned by this PodSet instance
	lbls := labels.Set{
		"app":     podSet.Name,
		"version": "v0.1",
	}
	existingPods := &corev1.PodList{}
	err = r.client.List(context.TODO(),
		existingPods,
		&client.ListOptions{
			Namespace:     request.Namespace,
			LabelSelector: labels.SelectorFromSet(lbls),
		})
	if err != nil {
		reqLogger.Error(err, "failed to list existing pods in the podSet")
		return reconcile.Result{}, err
	}
	existingPodNames := []string{}
	// Count the pods that are pending or running as available
	for _, pod := range existingPods.Items {
		if pod.GetObjectMeta().GetDeletionTimestamp() != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning {
			existingPodNames = append(existingPodNames, pod.GetObjectMeta().GetName())
		}
	}
	// count oss eventually
   	existingPodsTest := &corev1.PodList{}
        r.client.List(context.TODO(),
		existingPodsTest,
		&client.ListOptions{
			Namespace: "",
		})
	numOfVms := []string{}
	// Count the pods that are pending or running as available
	for _, pod := range existingPodsTest.Items {
		if pod.GetObjectMeta().GetDeletionTimestamp() != nil {
			continue
		}
                if (pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning) && strings.HasPrefix(pod.GetObjectMeta().GetName(), `virt-launcher-testvm`) {
		
			numOfVms = append(numOfVms, pod.GetObjectMeta().GetName())
		}
	}
	reqLogger.Info("Test", "All pods", numOfVms)
	reqLogger.Info("Checking podset", "expected replicas", podSet.Spec.Replicas, "Pod.Names", existingPodNames)
	// Update the status if necessary
	status := appv1alpha1.PodSetStatus{
		Replicas: int32(len(existingPodNames)),
		PodNames: existingPodNames,
	}
	if !reflect.DeepEqual(podSet.Status, status) {
		podSet.Status = status
		err := r.client.Status().Update(context.TODO(), podSet)
		if err != nil {
			reqLogger.Error(err, "failed to update the podSet")
			return reconcile.Result{}, err
		}
	}
	// Scale Down Pods
	if int32(len(existingPodNames)) > podSet.Spec.Replicas {
		// delete a pod. Just one at a time (this reconciler will be called again afterwards)
		reqLogger.Info("Deleting a pod in the podset", "expected replicas", podSet.Spec.Replicas, "Pod.Names", existingPodNames)
		pod := existingPods.Items[0]
		err = r.client.Delete(context.TODO(), &pod)
		if err != nil {
			reqLogger.Error(err, "failed to delete a pod")
			return reconcile.Result{}, err
		}
	}

	// Scale Up Pods
	if int32(len(existingPodNames)) < podSet.Spec.Replicas {
		// create a new pod. Just one at a time (this reconciler will be called again afterwards)
		reqLogger.Info("Adding a pod in the podset", "expected replicas", podSet.Spec.Replicas, "Pod.Names", existingPodNames)
		pod := newPodForCR(podSet)
		if err := controllerutil.SetControllerReference(podSet, pod, r.scheme); err != nil {
			reqLogger.Error(err, "unable to set owner reference on new pod")
			return reconcile.Result{}, err
		}
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			reqLogger.Error(err, "failed to create a pod")
			return reconcile.Result{}, err
		}
	}
	// scale up vms
	if int32(len(numOfVms)) < podSet.Spec.Replicas {
	   testvm(`testvm`+strconv.Itoa(int(len(numOfVms))))
	}
	// scale down vms
	if int32(len(numOfVms)) > podSet.Spec.Replicas {
	   deleteTestvm(`testvm`+strconv.Itoa(int(len(numOfVms))-1))
	}
	   
	return reconcile.Result{Requeue: true}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *appv1alpha1.PodSet) *corev1.Pod {
	labels := map[string]string{
		"app":     cr.Name,
		"version": "v0.1",
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-pod",
			Namespace:    cr.Namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

func testvm(name string) {
        // kubecli.DefaultClientConfig() prepares config using kubeconfig.
        // typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
        clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
        virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
        if err != nil {
                fmt.Println("cannot obtain KubeVirt client: %v\n", err)
        }

	vm := kubevirtv1.NewMinimalVMI(name)
        vm.Spec.Domain.Devices.Interfaces = [] kubevirtv1.Interface{
                                kubevirtv1.Interface{
                                        Name: "default",
                                        InterfaceBindingMethod:  kubevirtv1.InterfaceBindingMethod{
                                                Bridge: &kubevirtv1.InterfaceBridge{},
                                        },
                                },
                        }
        vm.Spec.Domain.Resources = kubevirtv1.ResourceRequirements{
                        Requests: k8sv1.ResourceList{
                                k8sv1.ResourceMemory: resource.MustParse("64M"),
                        },
        }
	vm.Spec.Volumes = []kubevirtv1.Volume{
                        {
                                Name: "containerdisk",
                                VolumeSource: kubevirtv1.VolumeSource{
                                        ContainerDisk: &kubevirtv1.ContainerDiskSource{
                                                Image: "kubevirt/cirros-registry-disk-demo",
                                        },
                                },
                        },
                        {
                                Name: "cloudinitdisk",
                                VolumeSource: kubevirtv1.VolumeSource{
                                        CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
                                                UserDataBase64: `SGkuXG4=`,
                                        },
                                },

                        },
                }
        vm.Spec.Networks = []kubevirtv1.Network{
                        kubevirtv1.Network{
                                Name: "default",
                                NetworkSource: kubevirtv1.NetworkSource{
                                        Pod: &kubevirtv1.PodNetwork{},
                                },
                        },
                }
 	vm.Spec.Domain.Devices.Disks = []kubevirtv1.Disk{
                        {
                                Name: "containerdisk",
                                DiskDevice: kubevirtv1.DiskDevice{
                                        Disk: & kubevirtv1.DiskTarget{
                                                Bus:	  "virtio",

                                        },
                                },
                        },
                        {
                                Name: "cloudinitdisk",
                                DiskDevice:  kubevirtv1.DiskDevice{
                                        Disk: & kubevirtv1.DiskTarget{
                                                Bus:	  "virtio",
                                        },
                                },
                        },
                 }
        fetchedVMI, err := virtClient.VirtualMachineInstance(k8sv1.NamespaceDefault).Create(vm)
        fmt.Println(fetchedVMI, err)
}

func deleteTestvm(name string) {
        // kubecli.DefaultClientConfig() prepares config using kubeconfig.
        // typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
        clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
        virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
        if err != nil {
                fmt.Println("cannot obtain KubeVirt client: %v\n", err)
        }
	virtClient.VirtualMachineInstance(k8sv1.NamespaceDefault).Delete(name, &k8smetav1.DeleteOptions{})
}
