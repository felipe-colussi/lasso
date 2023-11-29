package controller

import (
	"bytes"
	"context"
	"fmt"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
)

func init() {

	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		rancher = "agent"

		fmt.Println("Not Found Kubeconfig Path, trying new RKE2 one")
		config, err = clientcmd.BuildConfigFromFlags("", "/etc/rancher/rke2/rke2.yaml")
		if err != nil {
			fmt.Println("wasn't able to start from RKE2 path, trying k3s")
			config, err = clientcmd.BuildConfigFromFlags("", "/etc/rancher/k3s/k3s.yaml")
			rancher = "agent"
			if err != nil {
				fmt.Println("skipping kubernetes logs")
				return
			}

		}

	}

	// Setup types you want to work with
	scheme := runtime.NewScheme()
	appsv1.AddToScheme(scheme)

	controllerFactory, err := NewSharedControllerFactoryFromConfig(config, scheme)
	if err != nil {
		panic(err)
	}

	// ForObject, ForKind, ForResource, and ForResourceKind can all be used to retrieve the controller.
	// They all accept different parameters but the first three all eventually call ForResourceKind.
	// Refer to [How to use Lasso](#how-to-use-lasso) for more information on how ForResourceKind works.
	sharedController := controllerFactory.ForResourceKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}.
		GroupVersion().WithResource("configmaps"), "ConfigMap", true)
	if err != nil {
		panic(err)
	}

	// Register a handler on a shared controller. If you need a dedicated queue then use controller.New()
	sharedController.RegisterHandler(context.TODO(), "tracer-handler", SharedControllerHandlerFunc(func(key string, obj runtime.Object) (runtime.Object, error) {
		// obj is the latest version of this obj in the cache and MAY BE NIL when an object is finally deleted

		uns, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return obj, nil
		}

		var configMap corev1.ConfigMap
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uns.Object, &configMap); err != nil {
			panic(err)
		}

		if v, e := configMap.Labels["LOGGER"]; !e || v == "ignore" {
			return obj, nil
		}

		cp := configMap.DeepCopy()

		// Do some stuff ...

		if v, e := cp.Annotations["log"]; e && v == rancher {
			b := bytes.Buffer{}
			if err := debugMap.ToCsv(&b, rancher); err != nil {
				fmt.Println("ERROR Writing CSV: %s", err.Error())
				return nil, err
			}
			cp.Data[rancher] = b.String()
			cp.Annotations["log"] = "done"
			fmt.Println("Logs Created")

			if err := sharedController.Client().Update(context.TODO(), cp.Namespace, cp, cp, metav1.UpdateOptions{}); err != nil {
				return obj, err
			}

		}

		if v, e := cp.Annotations["DEBUG_LASSO_KIND"]; e && v != "" && v != "ignore" {
			registeredOn = v
			fmt.Printf("registeredOn Updated to: %s\n", v)
		}

		// Get stuff from the cache

		// return the latest version of the object
		return cp, nil
	}))

	// the second, int parameter is the number of workers to be used for EACH controller.
	err = controllerFactory.Start(context.TODO(), 1)
	if err != nil {
		panic(err)
	}
}
