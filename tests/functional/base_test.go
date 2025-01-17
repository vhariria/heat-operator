/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functional

import (
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	routev1 "github.com/openshift/api/route/v1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
)

func GetTransportURL(name types.NamespacedName) *rabbitmqv1.TransportURL {
	instance := &rabbitmqv1.TransportURL{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func CreateUnstructured(rawObj map[string]interface{}) *unstructured.Unstructured {
	logger.Info("Creating", "raw", rawObj)
	unstructuredObj := &unstructured.Unstructured{Object: rawObj}
	_, err := controllerutil.CreateOrPatch(
		ctx, k8sClient, unstructuredObj, func() error { return nil })
	Expect(err).ShouldNot(HaveOccurred())
	return unstructuredObj
}

func GetDefaultHeatSpec() map[string]interface{} {
	return map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           SecretName,
		"heatEngine":       GetDefaultHeatEngineSpec(),
		"heatAPI":          GetDefaultHeatAPISpec(),
		"heatCfnAPI":       GetDefaultHeatCFNAPISpec(),
	}
}

func GetDefaultHeatAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":         SecretName,
		"replicas":       1,
		"containerImage": "quay.io/podified-antelope-centos9/openstack-heat-api:current-podified",
	}
}

func GetDefaultHeatEngineSpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":         SecretName,
		"replicas":       1,
		"containerImage": "quay.io/podified-antelope-centos9/openstack-heat-engine:current-podified",
	}
}

func GetDefaultHeatCFNAPISpec() map[string]interface{} {
	return map[string]interface{}{
		"secret":         SecretName,
		"replicas":       1,
		"containerImage": "quay.io/podified-antelope-centos9/openstack-heat-cfn:current-podified",
	}
}

func CreateHeat(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "heat.openstack.org/v1beta1",
		"kind":       "Heat",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return CreateUnstructured(raw)

	// return types.NamespacedName{Name: HeatName, Namespace: namespace}
}

func DeleteInstance(instance client.Object) {
	// We have to wait for the controller to fully delete the instance
	logger.Info("Deleting", "Name", instance.GetName(), "Namespace", instance.GetNamespace(), "Kind", instance.GetObjectKind().GroupVersionKind().Kind)
	Eventually(func(g Gomega) {
		name := types.NamespacedName{Name: instance.GetName(), Namespace: instance.GetNamespace()}
		err := k8sClient.Get(ctx, name, instance)
		// if it is already gone that is OK
		if k8s_errors.IsNotFound(err) {
			return
		}
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(k8sClient.Delete(ctx, instance)).Should(Succeed())

		err = k8sClient.Get(ctx, name, instance)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func GetHeat(name types.NamespacedName) *heatv1.Heat {
	instance := &heatv1.Heat{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func CreateSecret(name types.NamespacedName, data map[string][]byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Data: data,
	}
	Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
	return secret
}

func CreateHeatSecret(namespace string, name string) *corev1.Secret {
	return CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{
			"HeatPassword":         []byte("12345678"),
			"HeatDatabasePassword": []byte("12345678"),
		},
	)
}

func CreateHeatMessageBusSecret(namespace string, name string) *corev1.Secret {
	return CreateSecret(
		types.NamespacedName{Namespace: namespace, Name: name},
		map[string][]byte{
			"transport_url": []byte("rabbit://fake"),
		},
	)
}

func HeatConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetHeat(name)
	return instance.Status.Conditions
}

// CreateKeystoneAPI initializes the the input for a KeystoneAPI instance
func CreateKeystoneAPI(namespace string) types.NamespacedName {
	keystone := &keystonev1.KeystoneAPI{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "keystone.openstack.org/v1beta1",
			Kind:       "KeystoneAPI",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-" + uuid.New().String(),
			Namespace: namespace,
		},
		Spec: keystonev1.KeystoneAPISpec{},
	}

	Expect(k8sClient.Create(ctx, keystone.DeepCopy())).Should(Succeed())
	name := types.NamespacedName{Namespace: namespace, Name: keystone.Name}

	// the Status field needs to be written via a separate client
	keystone = GetKeystoneAPI(name)
	keystone.Status = keystonev1.KeystoneAPIStatus{
		APIEndpoints: map[string]string{
			"admin":    "http://keystone-admin-openstack.testing",
			"internal": "http://keystone-internal-openstack.testing",
			"public":   "http://keystone-public-openstack.testing",
		},
	}
	Expect(k8sClient.Status().Update(ctx, keystone.DeepCopy())).Should(Succeed())

	logger.Info("KeystoneAPI created", "KeystoneAPI", name)
	return name
}

// DeleteKeystoneAPI Deletes KeystoneAPI instance
func DeleteKeystoneAPI(name types.NamespacedName) {
	Eventually(func(g Gomega) {
		keystone := &keystonev1.KeystoneAPI{}
		err := k8sClient.Get(ctx, name, keystone)
		// if it is already gone that is OK
		if k8s_errors.IsNotFound(err) {
			return
		}
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(k8sClient.Delete(ctx, keystone)).Should(Succeed())

		err = k8sClient.Get(ctx, name, keystone)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// GetKeystoneAPI Gets KeystoneAPI Instance
func GetKeystoneAPI(name types.NamespacedName) *keystonev1.KeystoneAPI {
	instance := &keystonev1.KeystoneAPI{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GetKeystoneService(name types.NamespacedName) *keystonev1.KeystoneService {
	instance := &keystonev1.KeystoneService{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func GetKeystoneEndpoint(name types.NamespacedName) *keystonev1.KeystoneEndpoint {
	instance := &keystonev1.KeystoneEndpoint{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func AssertServiceExists(name types.NamespacedName) *corev1.Service {
	instance := &corev1.Service{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func AssertRouteExists(name types.NamespacedName) *routev1.Route {
	instance := &routev1.Route{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// CreateDBService creates a k8s Service object that matches with the
// expectations of lib-common database module as a Service for the MariaDB
func CreateDBService(namespace string, mariadbCRName string, spec corev1.ServiceSpec) types.NamespacedName {
	// The Name is used as the hostname to access the service. So
	// we generate something unique for the MariaDB CR it represents
	// so we can assert that the correct Service is selected.
	//serviceName := "mariadb-" + mariadbCRName
	serviceName := "heat"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			// NOTE(gibi): The lib-common databvase module looks up the
			// Service exposed by MariaDB via these labels.
			Labels: map[string]string{
				"app": "mariadb",
				"cr":  "mariadb-" + mariadbCRName,
			},
		},
		Spec: spec,
	}
	Expect(k8sClient.Create(ctx, service)).Should(Succeed())

	return types.NamespacedName{Name: serviceName, Namespace: namespace}
}

func DeleteDBService(name types.NamespacedName) {
	Eventually(func(g Gomega) {
		service := &corev1.Service{}
		err := k8sClient.Get(ctx, name, service)
		// if it is already gone that is OK
		if k8s_errors.IsNotFound(err) {
			return
		}
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(k8sClient.Delete(ctx, service)).Should(Succeed())

		err = k8sClient.Get(ctx, name, service)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func GetMariaDBDatabase(name types.NamespacedName) *mariadbv1.MariaDBDatabase {
	instance := &mariadbv1.MariaDBDatabase{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func ListMariaDBDatabase(namespace string) *mariadbv1.MariaDBDatabaseList {
	mariaDBDatabases := &mariadbv1.MariaDBDatabaseList{}
	Expect(k8sClient.List(ctx, mariaDBDatabases, client.InNamespace(namespace))).Should(Succeed())
	return mariaDBDatabases
}
