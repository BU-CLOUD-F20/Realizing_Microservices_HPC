package podset

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
        
        "os/user"
	"io/ioutil"
	"golang.org/x/crypto/ssh"
		
	appv1alpha1 "podset-operator/pkg/apis/app/v1alpha1"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/kubecli"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
		if (pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning) && strings.HasPrefix(pod.GetObjectMeta().GetName(), `virt-launcher-lustre-oss`) {

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
	if podSet.Spec.Replicas <= 2 {
		if int32(len(numOfVms)) < podSet.Spec.Replicas {
			ossvm(`lustre-oss`+strconv.Itoa(int(len(numOfVms))), int(len(numOfVms)))
		}
	}
	// scale down vms
	if int32(len(numOfVms)) > podSet.Spec.Replicas {
		deleteTestvm(`lustre-oss` + strconv.Itoa(int(len(numOfVms))-1))
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
	vm.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
		kubevirtv1.Interface{
			Name: "default",
			InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
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
		{
			Name: "cloudinitdisk2",
			VolumeSource: kubevirtv1.VolumeSource{
				CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
					UserData: `|-
#cloud-config
runcmd:
- "mkdir /home/centos/test"
`,
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
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name: "cloudinitdisk",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name: "cloudinitdisk2",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
	}
	fetchedVMI, err := virtClient.VirtualMachineInstance(k8sv1.NamespaceDefault).Create(vm)
	fmt.Println(fetchedVMI, err)
}

func ossvm(name string, number int) {
	// kubecli.DefaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		fmt.Println("cannot obtain KubeVirt client: %v\n", err)
	}

	vm := kubevirtv1.NewMinimalVMI(name)
	vm.Spec.Domain.Devices.Interfaces = []kubevirtv1.Interface{
		kubevirtv1.Interface{
			Name: "default",
			InterfaceBindingMethod: kubevirtv1.InterfaceBindingMethod{
				Bridge: &kubevirtv1.InterfaceBridge{},
			},
		},
	}
	vm.Spec.Domain.Resources = kubevirtv1.ResourceRequirements{
		Requests: k8sv1.ResourceList{
			k8sv1.ResourceMemory: resource.MustParse("1024M"),
		},
	}
	vm.Spec.Volumes = []kubevirtv1.Volume{
		{
			Name: `vol-oss` + strconv.Itoa(number*2+1),
			VolumeSource: kubevirtv1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: `vol-oss` + strconv.Itoa(number*2+1),
				},
			},
		},
		{
			Name: `vol-oss` + strconv.Itoa(number*2+2),
			VolumeSource: kubevirtv1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: `vol-oss` + strconv.Itoa(number*2+2),
				},
			},
		},
		{
			Name: "containerdisk",
			VolumeSource: kubevirtv1.VolumeSource{
				ContainerDisk: &kubevirtv1.ContainerDiskSource{
					Image: "nakulvr/centos:lustre-server",
				},
			},
		},
		{
			Name: "cloudinitdisk",
			VolumeSource: kubevirtv1.VolumeSource{
				CloudInitNoCloud: &kubevirtv1.CloudInitNoCloudSource{
					UserData: `#cloud-config
ssh_authorized_keys:
  - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDAEIJZRVfM/sxhpR4jT6rwUNMZEarTPjhKDOn7ifZa+qa/4MHSCvcPvq0781zypZp6QnNW9WrALfsmi8QQg3P/74EHyNs/rBdFsKmvOsC//AcogRIynL+oR8AlLgs5fwntLEg3/6L9WLSimi5jjFF8QXMULtjfUypHA/6xvX/OHN+62D+ySYn9GeFFYeutUB+NLalvzsjlDTHt4dXmSy+wDM8tIetTIDuc2+yS6qv6tWiV1qaXCn3fHsKTZTnsNCcH9mVLv5CmIqkV/XpcbGqvn3unaBWn4uGwCyudjHM99HunCPbXBfJO4NiCxtFKVpV9wONC/se6KK32AUzd9q+ln3uNLwMaE9XMVpoxI1eE+UUQnRPwIyY9kKOtzbIutcIJmNJdC5xKpZa+tAoho3sHBdGUBpHBAARVwsYZj8S6Uv7jbsB0qDK+j19Dy9cb6E8oqpSj9WPqKsTI0be+nbzP+BvTvLXktp5s2JWuWPtl5OZOUDRv2boY831MIhDdvo0= centos@node-2
runcmd:
  - sudo exec /sbin/modprobe -v lnet >/dev/null 2>&1
  - /sbin/lsmod | /bin/grep lustre 1>/dev/null 2>&1
  - sudo /sbin/modprobe -v lustre >/dev/null 2>&1
  - /sbin/lsmod | /bin/grep zfs 1>/dev/null 2>&1
  - sudo /sbin/modprobe -v zfs >/dev/null 2>&1
  - sudo /usr/sbin/mkfs.lustre --ost --fsname=lustrefs --mgsnode=lustre-mgs.default-lustre@tcp0 --index=` + strconv.Itoa(number*2+1) + `--reformat --replace /dev/vdb > /dev/null 2>&1
  - sudo /usr/sbin/mkfs.lustre --ost --fsname=lustrefs --mgsnode=lustre-mgs.default-lustre@tcp0 --index=` + strconv.Itoa(number*2+2) + `--reformat --replace /dev/vdc > /dev/null 2>&1
  - sudo /usr/bin/mkdir /ost` + strconv.Itoa(number*2+1) + `
  - sudo /usr/bin/mkdir /ost` + strconv.Itoa(number*2+2) + `
  - sudo /usr/sbin/mount.lustre /dev/vdb /ost` + strconv.Itoa(number*2+1) + `
  - sudo /usr/sbin/mount.lustre /dev/vdc /ost` + strconv.Itoa(number*2+2),
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
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name: `vol-oss` + strconv.Itoa(number*2+1),
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name: `vol-oss` + strconv.Itoa(number*2+2),
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
				},
			},
		},
		{
			Name: "cloudinitdisk",
			DiskDevice: kubevirtv1.DiskDevice{
				Disk: &kubevirtv1.DiskTarget{
					Bus: "virtio",
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

func runStartupScript(user string, hostIP string, port string, privKeyFileName string, pubKeyFileName string){
	privKey, pubKey, err := getKeyFiles(privKeyFileName, pubKeyFileName)
	if err !=nil {
		panic(err)
        }

        // TODO: Here we should probably create the mounting points 

        var host string = hostIP + ":" + port
        client, session, err := connectToHost(user, host, privKey)
	if err != nil {
		panic(err)
	}

	out, err := session.CombinedOutput("echo \"" + user + ":centos\" | chpasswd")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo mkdir /home/" + user + "/.ssh")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo echo " + pubKey + " > /home/" + user + "/.ssh/authorized_keys")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo chown -R " + user + ": /home/" + user + "/.ssh")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo exec /sbin/modprobe -v lnet >/dev/null 2>&1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /sbin/modprobe -v lustre >/dev/null 2>&1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /sbin/modprobe -v zfs >/dev/null 2>&1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /usr/sbin/mkfs.lustre --ost --fsname=lustrefs --mgsnode=lustre-mgs.default-lustre@tcp0 --index=1 /dev/vdb > /dev/null 2>&1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /usr/sbin/mkfs.lustre --ost --fsname=lustrefs --mgsnode=lustre-mgs.default-lustre@tcp0 --index=2 /dev/vdc > /dev/null 2>&1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /usr/bin/mkdir /ost1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /usr/bin/mkdir /ost2")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /usr/sbin/mount.lustre /dev/vdb /ost1")
	if err != nil {
		panic(err)
        }
        
        out, err = session.CombinedOutput("sudo /usr/sbin/mount.lustre /dev/vdc /ost2")
	if err != nil {
		panic(err)
	}
        fmt.Println(string(out))
        
	client.Close()
}

func getKeyFiles(privKeyFileName string, pubKeyFilename string) (privKey ssh.Signer, pubKey string, err error){
        usr, _ := user.Current()
        file := usr.HomeDir + "/.ssh/" + privKeyFileName
        buf, err := ioutil.ReadFile(file)
        if err != nil {
                return
        }
        privKey, err = ssh.ParsePrivateKey(buf)
        file = usr.HomeDir + "/.ssh/" + pubKeyFilename
        buf, err = ioutil.ReadFile(file)
        pubKey = string(buf)
	fmt.Println("PubKey retrieved: " + string(buf))
        if err != nil {
                return
        }
        return
}

func connectToHost(user, host string, key ssh.Signer) (*ssh.Client, *ssh.Session, error) {
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	return client, session, nil
}
