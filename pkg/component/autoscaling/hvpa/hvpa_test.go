// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hvpa_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/autoscaling/hvpa"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("HVPA", func() {
	var (
		ctx = context.Background()

		namespace         = "some-namespace"
		image             = "some-image:some-tag"
		priorityClassName = "some-priority-class"
		values            = Values{
			Image:             image,
			PriorityClassName: priorityClassName,
			KubernetesVersion: semver.MustParse("1.25.5"),
		}

		c         client.Client
		component component.DeployWaiter
		consistOf func(object ...client.Object) types.GomegaMatcher

		managedResourceName   = "hvpa"
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		serviceAccount         *corev1.ServiceAccount
		clusterRole            *rbacv1.ClusterRole
		clusterRoleBinding     *rbacv1.ClusterRoleBinding
		service                *corev1.Service
		deployment             *appsv1.Deployment
		role                   *rbacv1.Role
		roleBinding            *rbacv1.RoleBinding
		vpa                    *vpaautoscalingv1.VerticalPodAutoscaler
		podDisruptionBudgetFor func(bool) *policyv1.PodDisruptionBudget
		serviceMonitor         *monitoringv1.ServiceMonitor
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		component = New(c, namespace, values)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		serviceAccount = &corev1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller",
				Namespace: namespace,
				Labels:    map[string]string{"gardener.cloud/role": "hvpa"},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "system:hvpa-controller",
				Labels: map[string]string{"gardener.cloud/role": "hvpa"},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "replicationcontrollers"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling"},
					Resources: []string{"horizontalpodautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"hvpas"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"hvpas/status"},
					Verbs:     []string{"get", "patch", "update"},
				},
				{
					APIGroups: []string{"autoscaling.k8s.io"},
					Resources: []string{"verticalpodautoscalers"},
					Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "hvpa-controller-rolebinding",
				Labels: map[string]string{"gardener.cloud/role": "hvpa"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "system:hvpa-controller",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "hvpa-controller",
				Namespace: namespace,
			}},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"hvpa-controller"},
					Verbs:         []string{"get", "watch", "update"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "get", "list", "watch", "patch"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      role.Name,
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}
		service = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller",
				Namespace: namespace,
				Labels: map[string]string{
					"gardener.cloud/role": "hvpa",
					"app":                 "hvpa-controller",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":9569}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:            corev1.ServiceTypeClusterIP,
				SessionAffinity: corev1.ServiceAffinityNone,
				Selector:        map[string]string{"app": "hvpa-controller"},
				Ports: []corev1.ServicePort{{
					Name:       "metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       9569,
					TargetPort: intstr.FromInt32(9569),
				}},
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller",
				Namespace: namespace,
				Labels: map[string]string{
					"app":                 "hvpa-controller",
					"gardener.cloud/role": "hvpa",
					"high-availability-config.resources.gardener.cloud/type": "controller",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"gardener.cloud/role": "hvpa",
						"app":                 "hvpa-controller",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"gardener.cloud/role":              "hvpa",
							"app":                              "hvpa-controller",
							"networking.gardener.cloud/to-dns": "allowed",
							"networking.gardener.cloud/to-runtime-apiserver": "allowed",
						},
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  priorityClassName,
						ServiceAccountName: serviceAccount.Name,
						Containers: []corev1.Container{{
							Name:            "hvpa-controller",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"./manager",
								"--logtostderr=true",
								"--leader-elect=true",
								"--enable-detailed-metrics=true",
								"--metrics-bind-address=:9569",
								"--v=2",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("500Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: 9569,
							}},
						}},
					},
				},
			},
		}

		maxUnavailable := intstr.FromInt32(1)
		podDisruptionBudgetFor = func(k8sVersionGreaterEquals126 bool) *policyv1.PodDisruptionBudget {
			podDisruptionBudget := &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hvpa-controller",
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "hvpa-controller",
						"gardener.cloud/role": "hvpa",
					},
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MaxUnavailable: &maxUnavailable,
					Selector:       deployment.Spec.Selector,
				},
			}

			unhealthyPodEvictionPolicyAlwatysAllow := policyv1.AlwaysAllow
			if k8sVersionGreaterEquals126 {
				podDisruptionBudget.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwatysAllow
			}

			return podDisruptionBudget
		}

		vpaUpdateMode := vpaautoscalingv1.UpdateModeAuto
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hvpa-controller-vpa",
				Namespace: namespace,
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       "hvpa-controller",
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
		}

		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cache-hvpa-controller",
				Namespace: namespace,
				Labels:    map[string]string{"prometheus": "cache"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				Selector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "hvpa-controller"}},
				Endpoints: []monitoringv1.Endpoint{{
					Port: "metrics",
					MetricRelabelConfigs: []*monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"__name__"},
						Action:       "keep",
						Regex:        `^(hvpa_aggregate_applied_scaling_total|hvpa_aggregate_blocked_scalings_total|hvpa_spec_replicas|hvpa_status_replicas|hvpa_status_applied_hpa_current_replicas|hvpa_status_applied_hpa_desired_replicas|hvpa_status_applied_vpa_recommendation|hvpa_status_blocked_hpa_current_replicas|hvpa_status_blocked_hpa_desired_replicas|hvpa_status_blocked_vpa_recommendation)$`,
					}},
				}},
			},
		}
	})

	JustBeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var expectedObjects []client.Object

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))
			expectedObjects = []client.Object{
				serviceAccount,
				clusterRole,
				clusterRoleBinding,
				service,
				deployment,
				role,
				roleBinding,
				vpa,
				serviceMonitor,
			}

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

		})

		Context("Kubernetes versions < 1.26", func() {
			It("should successfully deploy all resources", func() {
				expectedObjects = append(expectedObjects, podDisruptionBudgetFor(false))
				Expect(managedResource).To(consistOf(expectedObjects...))
			})
		})

		Context("Kubernetes versions >= 1.26", func() {
			BeforeEach(func() {
				values.KubernetesVersion = semver.MustParse("1.26.2")
				component = New(c, namespace, values)
			})

			It("should successfully deploy all resources", func() {
				expectedObjects = append(expectedObjects, podDisruptionBudgetFor(true))
				Expect(managedResource).To(consistOf(expectedObjects...))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				managedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
					},
				}

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
