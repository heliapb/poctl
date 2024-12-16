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

package analyzers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus-operator/poctl/internal/k8sutil"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func RunPrometheusAgentAnalyzer(ctx context.Context, clientSets *k8sutil.ClientSets, name, namespace string) error {
	prometheusagent, err := clientSets.MClient.MonitoringV1alpha1().PrometheusAgents(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("prometheus %s not found in namespace %s", name, namespace)
		}
		return fmt.Errorf("error while getting Prometheus: %v", err)
	}

	cRb, err := clientSets.KClient.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{
		LabelSelector: "name=prometheus-agent",
	})
	if err != nil {
		return fmt.Errorf("failed to list RoleBindings: %w", err)
	}

	if !k8sutil.IsServiceAccountBoundToRoleBindingList(cRb, prometheusagent.Spec.ServiceAccountName) {
		return fmt.Errorf("serviceAccount %s is not bound to any RoleBindings", prometheusagent.Spec.ServiceAccountName)
	}

	for _, crb := range cRb.Items {
		cr, err := clientSets.KClient.RbacV1().ClusterRoles().Get(ctx, crb.RoleRef.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get ClusterRole %s", crb.RoleRef.Name)
		}

		err = k8sutil.CheckPrometheusClusterRoleRules(crb, cr)
		if err != nil {
			return err
		}
	}

	slog.Info("Prometheus Agent is compliant, no issues found", "name", name, "namespace", namespace)
	return nil
}
