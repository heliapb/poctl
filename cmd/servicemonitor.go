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

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus-operator/poctl/internal/k8sutil"
	"github.com/prometheus-operator/poctl/internal/log"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/client/applyconfiguration/monitoring/v1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyConfigMetav1 "k8s.io/client-go/applyconfigurations/meta/v1"

	"k8s.io/utils/ptr"
)

var (
	serviceName       string
	namespace         string
	port              string
	servicemonitorCmd = &cobra.Command{
		Use:   "servicemonitor",
		Short: "Create a service monitor object",
		Long:  `Create a service monitor object based on user input parameters or taking as source of truth a kubernetes service`,
		RunE:  runServiceMonitor,
	}
)

func runServiceMonitor(_ *cobra.Command, _ []string) error {
	logger, err := log.NewLogger()
	if err != nil {
		fmt.Println(err)
		return err
	}

	clientSets, err := k8sutil.GetClientSets(kubeconfig)
	if err != nil {
		logger.Error("error while getting client sets", "err", err)
		return err
	}

	if serviceName == "" {
		logger.Error("service name is required")
		return errors.New("service name is required")
	}

	err = createFromService(context.Background(), clientSets, namespace, serviceName, port)
	if err != nil {
		logger.Error("error while creating service monitor", "err", err)
		return err
	}

	return nil
}

func createFromService(
	ctx context.Context,
	clientSets *k8sutil.ClientSets,
	namespace string,
	serviceName string,
	port string) error {

	service, err := clientSets.KClient.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error while getting service %s: %v", serviceName, err)
	}

	svcMonitor := &monitoringv1.ServiceMonitorApplyConfiguration{
		TypeMetaApplyConfiguration: applyConfigMetav1.TypeMetaApplyConfiguration{
			Kind:       ptr.To("ServiceMonitor"),
			APIVersion: ptr.To("monitoring.coreos.com/v1"),
		},
		ObjectMetaApplyConfiguration: &applyConfigMetav1.ObjectMetaApplyConfiguration{
			Name:      ptr.To(serviceName),
			Namespace: ptr.To(namespace),
			Labels:    service.Labels,
		},
		Spec: &monitoringv1.ServiceMonitorSpecApplyConfiguration{
			Selector: &applyConfigMetav1.LabelSelectorApplyConfiguration{
				MatchLabels: service.Spec.Selector,
			},
		},
	}

	for _, p := range service.Spec.Ports {
		if port != "" && p.Name != port {
			continue
		}

		svcMonitor.Spec.Endpoints = append(svcMonitor.Spec.Endpoints, monitoringv1.EndpointApplyConfiguration{
			HonorLabels: ptr.To(true),
			Port:        ptr.To(p.Name),
		})
	}

	_, err = clientSets.MClient.MonitoringV1().ServiceMonitors(namespace).Apply(ctx, svcMonitor, k8sutil.ApplyOption)
	if err != nil {
		return fmt.Errorf("error while creating service monitor %s: %v", serviceName, err)
	}

	return nil
}

func init() {
	createCmd.AddCommand(servicemonitorCmd)
	servicemonitorCmd.Flags().StringVarP(&serviceName, "service", "s", "", "Service name to create the service monitor from")
	servicemonitorCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Namespace of the service")
	servicemonitorCmd.Flags().StringVarP(&port, "port", "p", "", "Port of the service")
}
