package testingutil

import (
	"fmt"

	"github.com/redhat-developer/odo-fork/pkg/util"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getContainerPort(containerPort int32, containerProtocol corev1.Protocol) (container corev1.ContainerPort) {
	return corev1.ContainerPort{
		Name:          fmt.Sprintf("%v/%v", containerPort, containerProtocol),
		ContainerPort: containerPort,
		Protocol:      containerProtocol,
	}
}

func getContainer(componentName string, applicationName string, ports []corev1.ContainerPort,
	envFromSources []corev1.EnvFromSource) corev1.Container {
	return corev1.Container{
		Name:    fmt.Sprintf("%v-%v", componentName, applicationName),
		Image:   fmt.Sprintf("%v-%v", componentName, applicationName),
		Ports:   ports,
		EnvFrom: envFromSources,
	}
}

func getDeployment(namespace string, componentName string, componentType string, applicationName string, containers []corev1.Container) v1.Deployment {
	replicas := int32(1)
	return v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", componentName, applicationName),
			Namespace: namespace,
			Labels: map[string]string{
				"app":                              "app",
				"app.kubernetes.io/component-name": componentName,
				"app.kubernetes.io/component-type": componentType,
				"app.kubernetes.io/name":           applicationName,
			},
			Annotations: map[string]string{ // Convert into separate function when other source types required in tests
				"app.kubernetes.io/component-source-type": "git",
				"app.kubernetes.io/url":                   fmt.Sprintf("https://github.com/%s/%s.git", componentName, applicationName),
			},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"deploymentconfig": fmt.Sprintf("%v-%v", componentName, applicationName),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"deploymentconfig": fmt.Sprintf("%v-%v", componentName, applicationName),
					},
				},
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

func FakeDeployments() *v1.DeploymentList {

	var componentName string
	var applicationName string
	var componentType string

	// DC1 with multiple containers each with multiple ports
	componentType = "python"
	componentName = "python"
	applicationName = "app"
	c1 := getContainer(componentName, applicationName, []corev1.ContainerPort{
		{
			Name:          fmt.Sprintf("%v-%v-p1", componentName, applicationName),
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          fmt.Sprintf("%v-%v-p2", componentName, applicationName),
			ContainerPort: 9090,
			Protocol:      corev1.ProtocolUDP,
		},
	}, nil)
	c2 := getContainer(componentName, applicationName, []corev1.ContainerPort{
		{
			Name:          fmt.Sprintf("%v-%v-p1", componentName, applicationName),
			ContainerPort: 10080,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          fmt.Sprintf("%v-%v-p2", componentName, applicationName),
			ContainerPort: 10090,
			Protocol:      corev1.ProtocolUDP,
		},
	}, nil)
	dc1 := getDeployment("myproject", componentName, componentType, applicationName, []corev1.Container{c1, c2})

	// DC2 with single container and single port
	componentType = "nodejs"
	componentName = "nodejs"
	applicationName = "app"
	c3 := getContainer(componentName, applicationName, []corev1.ContainerPort{
		{
			Name:          fmt.Sprintf("%v-%v-p1", componentName, applicationName),
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
	}, []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "s1",
				},
			},
		},
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "s2",
				},
			},
		},
	})
	dc2 := getDeployment("myproject", componentName, componentType, applicationName, []corev1.Container{c3})

	// DC3 with single container and multiple ports
	componentType = "wildfly"
	componentName = "wildfly"
	applicationName = "app"
	c4 := getContainer(componentName, applicationName, []corev1.ContainerPort{
		{
			Name:          fmt.Sprintf("%v-%v-p1", componentName, applicationName),
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          fmt.Sprintf("%v-%v-p1", componentName, applicationName),
			ContainerPort: 8090,
			Protocol:      corev1.ProtocolTCP,
		},
	}, nil)
	dc3 := getDeployment("myproject", componentName, componentType, applicationName, []corev1.Container{c4})

	return &v1.DeploymentList{
		Items: []v1.Deployment{
			dc1,
			dc2,
			dc3,
		},
	}
}

// mountedStorage is the map of the storage to be mounted
// key is the path for the mount, value is the pvc
func OneFakeDeploymentWithMounts(componentName, componentType, applicationName string, mountedStorage map[string]*corev1.PersistentVolumeClaim) *v1.Deployment {
	c := getContainer(componentName, applicationName, []corev1.ContainerPort{
		{
			Name:          fmt.Sprintf("%v-%v-p1", componentName, applicationName),
			ContainerPort: 8080,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          fmt.Sprintf("%v-%v-p2", componentName, applicationName),
			ContainerPort: 9090,
			Protocol:      corev1.ProtocolUDP,
		},
	}, nil)

	dc := getDeployment("myproject", componentName, componentType, applicationName, []corev1.Container{c})

	supervisorDPVC := FakePVC(getAppRootVolumeName(dc.Name), "1Gi", nil)

	for path, pvc := range mountedStorage {
		volumeName := generateVolumeNameFromPVC(pvc.Name)
		dc.Spec.Template.Spec.Volumes = append(dc.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
				},
			},
		})
		dc.Spec.Template.Spec.Containers[0].VolumeMounts = append(dc.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: path,
		})
	}

	// now append the supervisorD volume
	dc.Spec.Template.Spec.Volumes = append(dc.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: getAppRootVolumeName(dc.Name),
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: supervisorDPVC.Name,
			},
		},
	})

	// now append the supervisorD volume mount
	dc.Spec.Template.Spec.Containers[0].VolumeMounts = append(dc.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      getAppRootVolumeName(dc.Name),
		MountPath: "/opt/app-root",
		SubPath:   "app-root",
	})

	return &dc
}

// generateVolumeNameFromPVC generates a random volume name based on the name
// of the given PVC
func generateVolumeNameFromPVC(pvc string) string {
	return fmt.Sprintf("%v-%v-volume", pvc, util.GenerateRandomString(5))
}

func getAppRootVolumeName(dcName string) string {
	return fmt.Sprintf("%s-idpdata", dcName)
}
