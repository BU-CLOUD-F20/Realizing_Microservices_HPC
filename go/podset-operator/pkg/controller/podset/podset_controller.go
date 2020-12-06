package podset

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	appv1alpha1 "podset-operator/pkg/apis/app/v1alpha1"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
var mgsIp string = ""
var mdsIp string = ""
var clientIp string = ""
var maxVmCount int = -1

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

	// get master ip
	// get the kubevirt client, using which kubevirt resources can be managed.
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})
	namespace, _, err := clientConfig.Namespace()
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)

	nodes, err := virtClient.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	nodeip := []corev1.NodeAddress{}
	for i := 0; i < len(nodes.Items); i++ {
		nodeip = nodes.Items[i].Status.Addresses
		fmt.Println(nodeip[0].Address)
	}

	fmt.Println(nodes.Items[0].Status.Addresses)

	vmiList, err := virtClient.VirtualMachineInstance(namespace).List(&k8smetav1.ListOptions{})
	if err != nil {
		fmt.Println("cannot obtain KubeVirt client: %v\n", err)
	}

	if err != nil {
		fmt.Println("cannot obtain KubeVirt client: %v\n", err)
	}
	for _, vmi := range vmiList.Items {
		if vmi.Name == `lustre-mds` {
			mdsIp = strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
			// reqLogger.Info("Test", "vmi ip", mdsIp)
		}
		if vmi.Name == `lustre-mgs` {
			mgsIp = strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
			// reqLogger.Info("Test", "vmi ip", mgsIp)
		}
		if vmi.Name == `lustre-client` {
			clientIp = strings.Split(vmi.Status.Interfaces[0].IP, "/")[0]
			// reqLogger.Info("Test", "vmi ip", clientIp)
		}
	}
	usage := getFsSize()

	if podSet.Spec.Replicas == int32(-1) {
		if usage >= 75 {
			fmt.Println("Time to scale up!\n")
			createPv(nodes.Items[0].Status.Addresses[0].Address, int(len(numOfVms)))
			createPvc(int(len(numOfVms)))
			enableOst(int(len(numOfVms)))
			ossvm(`lustre-oss`+strconv.Itoa(int(len(numOfVms))), int(len(numOfVms)))
		} else if (usage <= 20) && (int(len(numOfVms)) > 1) {
			fmt.Println("Time to scale down!\n")
			mergeOst(int(len(numOfVms)))
			deleteTestvm(`lustre-oss` + strconv.Itoa(int(len(numOfVms))-1))
			deletePvc(int(len(numOfVms)))
			deletePv(nodes.Items[0].Status.Addresses[0].Address, int(len(numOfVms)))
		} else {
			fmt.Println("Nothing to be done\n")
		}
		return reconcile.Result{Requeue: true, RequeueAfter: 60 * time.Second}, nil
	} else {
		// scale up vms
		if int32(len(numOfVms)) < podSet.Spec.Replicas {
			createPv(nodes.Items[0].Status.Addresses[0].Address, int(len(numOfVms)))
			createPvc(int(len(numOfVms)))
			enableOst(int(len(numOfVms)))
			ossvm(`lustre-oss`+strconv.Itoa(int(len(numOfVms))), int(len(numOfVms)))
		}
		// scale down vms
		if int32(len(numOfVms)) > podSet.Spec.Replicas {
			mergeOst(int(len(numOfVms)))
			deleteTestvm(`lustre-oss` + strconv.Itoa(int(len(numOfVms))-1))
			deletePvc(int(len(numOfVms)))
			deletePv(nodes.Items[0].Status.Addresses[0].Address, int(len(numOfVms)))
		}
		return reconcile.Result{Requeue: true}, nil
	}
}

func mergeOst(number int) {
	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	//mds commands
	client, session, err := connectToHost("centos", mdsIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	var b bytes.Buffer
	session.Stdout = &b
	commands := []string{
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa((number-1)*2+1) + `*.max_create_count=0`,
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa((number-1)+2+2) + `*.max_create_count=0`,
	}
	command := strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("MDS Failed to run: " + err.Error())
	}
	client.Close()
	session.Close()

	//client commands
	client, session, err = connectToHost("centos", clientIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	session.Stdout = &b
	commands = []string{
		`sudo lfs find --ost lustrefs-OST000` + strconv.Itoa((number-1)*2+1) + `_UUID /lustrefs | sudo lfs_migrate -y`,
		`sudo lfs find --ost lustrefs-OST000` + strconv.Itoa((number-1)*2+2) + `_UUID /lustrefs | sudo lfs_migrate -y`,
	}
	command = strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("number is ", number)
		fmt.Println("Client Failed to run: " + err.Error())
	}
	client.Close()
	session.Close()

	//mds disable commands
	client, session, err = connectToHost("centos", mdsIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	session.Stdout = &b
	commands = []string{
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa((number-1)*2+1) + `-*.active=0`,
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa((number-1)*2+2) + `-*.active=0`,
	}
	command = strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("mds disable failed to run: " + err.Error())
	}
	client.Close()
	session.Close()

	//client disable commands
	client, session, err = connectToHost("centos", clientIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	session.Stdout = &b
	commands = []string{
		`sudo lctl set_param osc.lustrefs-OST000` + strconv.Itoa((number-1)*2+1) + `-*.active=0`,
		`sudo lctl set_param osc.lustrefs-OST000` + strconv.Itoa((number-1)*2+2) + `-*.active=0`,
	}
	command = strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("number is ", number)
		fmt.Println("client disable failed to run: " + err.Error())
	}
	client.Close()
	session.Close()
}

func enableOst(number int) {
	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	//mds commands
	client, session, err := connectToHost("centos", mdsIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	var b bytes.Buffer
	session.Stdout = &b
	commands := []string{
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa(number*2+1) + `*.max_create_count=20000`,
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa(number*2+2) + `*.max_create_count=20000`,
	}
	command := strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("MDS max count enable failed to run: " + err.Error())
	}
	client.Close()
	session.Close()

	//mds enable commands
	client, session, err = connectToHost("centos", mdsIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	session.Stdout = &b
	commands = []string{
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa(number*2+1) + `-*.active=1`,
		`sudo lctl set_param osp.lustrefs-OST000` + strconv.Itoa(number*2+2) + `-*.active=1`,
	}
	command = strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("MGS active enable failed to run: " + err.Error())
	}
	client.Close()
	session.Close()

	//client enable commands
	client, session, err = connectToHost("centos", clientIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
		return
	}

	session.Stdout = &b
	commands = []string{
		`sudo lctl set_param osc.lustrefs-OST000` + strconv.Itoa(number*2+1) + `-*.active=1`,
		`sudo lctl set_param osc.lustrefs-OST000` + strconv.Itoa(number*2+2) + `-*.active=1`,
	}
	command = strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("client enable failed to run: " + err.Error())
	}
	client.Close()
	session.Close()
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

	var arg1 string = ""
	var arg2 string = ""
	count := getOspMeta()
	if number < count {
		arg1 = strconv.Itoa(number*2+1) + ` --reformat --replace `
		arg2 = strconv.Itoa(number*2+2) + ` --reformat --replace `
	} else {
		arg1 = strconv.Itoa(number*2 + 1)
		arg2 = strconv.Itoa(number*2 + 2)
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
  - sudo /usr/sbin/mkfs.lustre --ost --fsname=lustrefs --mgsnode=lustre-mgs.default-lustre@tcp0 --index=` + arg1 + ` /dev/vdb > /dev/null 2>&1
  - sudo /usr/sbin/mkfs.lustre --ost --fsname=lustrefs --mgsnode=lustre-mgs.default-lustre@tcp0 --index=` + arg2 + ` /dev/vdc > /dev/null 2>&1
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

func getOspMeta() int {
	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	//master node commands
	client, session, err := connectToHost("centos", mdsIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
	}

	var b bytes.Buffer
	session.Stdout = &b
	commands := []string{
		`sudo lctl dl | grep osp`,
	}
	command := strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("MDS count:" + err.Error())
	}
	client.Close()
	session.Close()

	s := string(b.Bytes())
	var count int = 0
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		count++
	}
	return count / 2
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

func getKeyFile() (key ssh.Signer, err error) {
	buf := `-----BEGIN OPENSSH PRIVATE KEY-----
enter your key here
-----END OPENSSH PRIVATE KEY-----
`
	var b []byte = []byte(buf)
	key, err = ssh.ParsePrivateKey(b)
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

func createPvc(number int) {
	var class *string
	var s string = "local-storage"
	class = &s
	pvc1 := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vol-oss" + strconv.Itoa(number*2+1),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(v1.ResourceStorage): resource.MustParse("1Gi"),
				},
			},
			StorageClassName: class,
		},
	}
	pvc2 := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vol-oss" + strconv.Itoa(number*2+2),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(v1.ResourceStorage): resource.MustParse("1Gi"),
				},
			},
			StorageClassName: class,
		},
	}
	// kubecli.DefaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	if err != nil {
		fmt.Println("cannot obtain KubeVirt client: %v\n", err)
	}
	_, err = virtClient.CoreV1().PersistentVolumeClaims(k8sv1.NamespaceDefault).Create(pvc1)
	fmt.Println(err)
	_, err = virtClient.CoreV1().PersistentVolumeClaims(k8sv1.NamespaceDefault).Create(pvc2)
	fmt.Println(err)
}

func deletePvc(number int) {
	// kubecli.DefaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	fmt.Println(err)
	_ = virtClient.CoreV1().PersistentVolumeClaims(k8sv1.NamespaceDefault).Delete("vol-oss"+strconv.Itoa((number-1)*2+1), &metav1.DeleteOptions{})
	_ = virtClient.CoreV1().PersistentVolumeClaims(k8sv1.NamespaceDefault).Delete("vol-oss"+strconv.Itoa((number-1)*2+2), &metav1.DeleteOptions{})
}

func createPv(ip string, number int) {
	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	//master node commands
	client, session, err := connectToHost("centos", ip+`:22`, key)
	if err != nil {
		fmt.Println(err)
	}

	var b bytes.Buffer
	session.Stdout = &b
	commands := []string{
		`sudo mkdir /pvc-data/ost` + strconv.Itoa(number*2+1),
		`sudo mkdir /pvc-data/ost` + strconv.Itoa(number*2+2),
	}
	command := strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("Master pvc created error:" + err.Error())
	}
	client.Close()
	session.Close()

	// kubecli.DefaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	mode := corev1.PersistentVolumeFilesystem
	pv1 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-oss" + strconv.Itoa(number*2+1),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				"storage": resource.MustParse("1Gi"),
			},
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			VolumeMode:       &mode,
			StorageClassName: "local-storage",
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: `/pvc-data/ost` + strconv.Itoa(number*2+1)},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      `kubernetes.io/hostname`,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{
										"master-node",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	pv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pv-oss" + strconv.Itoa(number*2+2),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				"storage": resource.MustParse("1Gi"),
			},
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			VolumeMode:       &mode,
			StorageClassName: "local-storage",
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: `/pvc-data/ost` + strconv.Itoa(number*2+2)},
			},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{
									Key:      `kubernetes.io/hostname`,
									Operator: corev1.NodeSelectorOpIn,
									Values: []string{
										"master-node",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = virtClient.CoreV1().PersistentVolumes().Create(pv1)
	fmt.Println(err)

	_, err = virtClient.CoreV1().PersistentVolumes().Create(pv2)
	fmt.Println(err)
}

func deletePv(ip string, number int) {
	// kubecli.DefaultClientConfig() prepares config using kubeconfig.
	// typically, you need to set env variable, KUBECONFIG=<path-to-kubeconfig>/.kubeconfig
	clientConfig := kubecli.DefaultClientConfig(&pflag.FlagSet{})

	// get the kubevirt client, using which kubevirt resources can be managed.
	virtClient, err := kubecli.GetKubevirtClientFromClientConfig(clientConfig)
	_ = virtClient.CoreV1().PersistentVolumes().Delete("pv-oss"+strconv.Itoa((number-1)*2+1), &metav1.DeleteOptions{})
	_ = virtClient.CoreV1().PersistentVolumes().Delete("pv-oss"+strconv.Itoa((number-1)*2+2), &metav1.DeleteOptions{})

	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	//master node commands
	client, session, err := connectToHost("centos", ip+`:22`, key)
	if err != nil {
		fmt.Println(err)
	}

	var b bytes.Buffer
	session.Stdout = &b
	commands := []string{
		`sudo rm -r /pvc-data/ost` + strconv.Itoa((number-1)*2+1),
		`sudo rm -r /pvc-data/ost` + strconv.Itoa((number-1)*2+2),
	}
	command := strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("Master pvc created error:" + err.Error())
	}
	client.Close()
	session.Close()
}

func getFsSize() int {
	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	//master node commands
	client, session, err := connectToHost("centos", clientIp+`:22`, key)
	if err != nil {
		fmt.Println(err)
	}

	var b bytes.Buffer
	session.Stdout = &b
	commands := []string{
		`sudo df -h /lustrefs`,
	}
	command := strings.Join(commands, "; ")
	if err := session.Run(command); err != nil {
		fmt.Println("Master pvc created error:" + err.Error())
	}
	client.Close()
	session.Close()

	s := string(b.Bytes())
	var usage int
	var count int = 0
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		count++
		if count == 2 {
			usage, _ = strconv.Atoi(strings.Fields(scanner.Text())[4][0 : len(strings.Fields(scanner.Text())[4])-1])
		}
	}

	return usage
}
