// Copyright 2024 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sutil

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	apiv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiExtensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var ApplyOption = metav1.ApplyOptions{
	FieldManager: "application/apply-patch",
}

func getKubeConfig() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	kubeConfig := filepath.Clean(fmt.Sprintf("%v/.kube/config", usr.HomeDir))

	if _, err := os.Stat(kubeConfig); err != nil {
		return "", err
	}

	return kubeConfig, nil
}

func GetRestConfig(kubeConfig string) (*rest.Config, error) {
	var config *rest.Config
	var err error

	if kubeConfig == "" {
		kubeConfig, err = getKubeConfig()
		if err != nil {
			return nil, fmt.Errorf("error while getting kubeconfig: %v", err)
		}
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating k8s client config: %v", err)
	}

	return config, nil
}

func CrdDeserilezer(logger *slog.Logger, reader io.ReadCloser) (runtime.Object, error) {
	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)
	_ = apiextv1beta1.AddToScheme(sch)
	_ = apiv1.AddToScheme(sch)

	_ = monitoringv1.AddToScheme(sch)
	_ = monitoringv1alpha1.AddToScheme(sch)

	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		logger.Error("error while reading CRD", "error", err)
		return &runtime.Unknown{}, err
	}

	decode := serializer.NewCodecFactory(sch).UniversalDeserializer().Decode

	obj, _, err := decode(buf.Bytes(), nil, nil)
	if err != nil {
		logger.Error("error while decoding CRD", "error", err)
		return &runtime.Unknown{}, err
	}

	return obj, nil
}

type ClientSets struct {
	KClient             kubernetes.Interface
	MClient             monitoringclient.Interface
	DClient             dynamic.Interface
	APIExtensionsClient apiExtensions.Interface
}

func GetClientSets(kubeconfig string) (*ClientSets, error) {
	restConfig, err := GetRestConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error while getting k8s client config: %v", err)

	}

	kclient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating k8s client: %v", err)
	}

	mclient, err := monitoringclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating Prometheus Operator client: %v", err)
	}

	kdynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating dynamic client: %v", err)
	}

	apiExtensions, err := apiExtensions.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating apiextensions client: %v", err)
	}

	return &ClientSets{
		KClient:             kclient,
		MClient:             mclient,
		DClient:             kdynamicClient,
		APIExtensionsClient: apiExtensions,
	}, nil
}
