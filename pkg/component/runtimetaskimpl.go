package component

import (
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/redhat-developer/odo-fork/pkg/build"
	"github.com/redhat-developer/odo-fork/pkg/kclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunTaskExec is the Run Task execution implementation of the IDP run task
func RunTaskExec(Client *kclient.Client, projectName string, fullBuild bool) error {
	clientset := Client.KubeClient
	namespace := Client.Namespace

	glog.V(0).Infof("Namespace: %s\n", namespace)

	idpClaimName := ""
	PVCs, err := Client.GetPVCsFromSelector("app=idp")
	if err != nil {
		glog.V(0).Infof("Error occured while getting the PVC")
		err = errors.New("Unable to get the PVC: " + err.Error())
		return err
	}
	if len(PVCs) == 1 {
		idpClaimName = PVCs[0].GetName()
	}
	glog.V(0).Infof("Persistent Volume Claim: %s\n", idpClaimName)

	serviceAccountName := "default"
	glog.V(0).Infof("Service Account: %s\n", serviceAccountName)

	// cwd is the project root dir, where udo command will run
	cwd, err := os.Getwd()
	if err != nil {
		err = errors.New("Unable to get the cwd" + err.Error())
		return err
	}
	glog.V(0).Infof("CWD: %s\n", cwd)

	// Create the Runtime Task Instance
	RuntimeTaskInstance := BuildTask{
		UseRuntime:         true,
		Kind:               ComponentType,
		Name:               strings.ToLower(projectName) + "-runtime",
		Image:              RuntimeContainerImageWithBuildTools,
		ContainerName:      RuntimeContainerName,
		Namespace:          namespace,
		PVCName:            "",
		ServiceAccountName: serviceAccountName,
		// OwnerReferenceName: ownerReferenceName,
		// OwnerReferenceUID:  ownerReferenceUID,
		Privileged: true,
		MountPath:  RuntimeContainerMountPathEmptyDir,
		SubPath:    "",
	}

	glog.V(0).Info("Checking if Runtime Container has already been deployed...\n")
	foundRuntimeContainer := false
	timeout := int64(10)
	watchOptions := metav1.ListOptions{
		LabelSelector:  "app=" + RuntimeTaskInstance.Name + "-selector,chart=" + RuntimeTaskInstance.Name + "-1.0.0,release=" + RuntimeTaskInstance.Name,
		TimeoutSeconds: &timeout,
	}
	po, _ := Client.WaitAndGetPod(watchOptions, corev1.PodRunning, "Checking to see if a Runtime Container has already been deployed")
	if po != nil {
		glog.V(0).Infof("Running pod found: %s...\n\n", po.Name)
		RuntimeTaskInstance.PodName = po.Name
		foundRuntimeContainer = true
	}

	if !foundRuntimeContainer {
		// Deploy the application if it is a full build type and a running pod is not found
		glog.V(0).Info("Deploying the application")

		RuntimeTaskInstance.Labels = map[string]string{
			"app":     RuntimeTaskInstance.Name + "-selector",
			"chart":   RuntimeTaskInstance.Name + "-1.0.0",
			"release": RuntimeTaskInstance.Name,
		}

		// Deploy Application
		deploy := CreateDeploy(RuntimeTaskInstance)
		service := CreateService(RuntimeTaskInstance)

		glog.V(0).Info("===============================")
		glog.V(0).Info("Deploying application...")
		_, err = clientset.CoreV1().Services(namespace).Create(&service)
		if err != nil {
			err = errors.New("Unable to create component service: " + err.Error())
			return err
		}
		glog.V(0).Info("The service has been created.")

		_, err = clientset.AppsV1().Deployments(namespace).Create(&deploy)
		if err != nil {
			err = errors.New("Unable to create component deployment: " + err.Error())
			return err
		}
		glog.V(0).Info("The deployment has been created.")
		glog.V(0).Info("===============================")

		// Wait for the pod to run
		glog.V(0).Info("Waiting for pod to run\n")
		watchOptions := metav1.ListOptions{
			LabelSelector: "app=" + RuntimeTaskInstance.Name + "-selector,chart=" + RuntimeTaskInstance.Name + "-1.0.0,release=" + RuntimeTaskInstance.Name,
		}
		po, err := Client.WaitAndGetPod(watchOptions, corev1.PodRunning, "Waiting for the Component Container to run")
		if err != nil {
			err = errors.New("The Component Container failed to run")
			return err
		}
		glog.V(0).Info("The Component Pod is up and running: " + po.Name)
		RuntimeTaskInstance.PodName = po.Name
	}

	watchOptions = metav1.ListOptions{
		LabelSelector: "app=" + RuntimeTaskInstance.Name + "-selector,chart=" + RuntimeTaskInstance.Name + "-1.0.0,release=" + RuntimeTaskInstance.Name,
	}
	err = syncProjectToRunningContainer(Client, watchOptions, cwd, RuntimeTaskInstance.MountPath+"/src", RuntimeTaskInstance.ContainerName)
	if err != nil {
		glog.V(0).Infof("Error occured while syncing to the pod %s: %s\n", RuntimeTaskInstance.PodName, err)
		err = errors.New("Unable to sync to the pod: " + err.Error())
		return err
	}

	task := RuntimeTaskInstance.MountPath + "/src" + build.FullRunTask
	if !fullBuild {
		task = RuntimeTaskInstance.MountPath + "/src" + build.IncrementalRunTask
	}

	err = executetask(Client, task, RuntimeTaskInstance.PodName)
	if err != nil {
		glog.V(0).Infof("Error occured while executing command %s in the pod %s: %s\n", task, RuntimeTaskInstance.PodName, err)
		err = errors.New("Unable to exec command " + task + " in the runtime container: " + err.Error())
		return err
	}

	return nil
}