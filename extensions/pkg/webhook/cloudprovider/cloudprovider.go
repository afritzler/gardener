// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	// WebhookName is the name of the webhook.
	WebhookName = "cloudprovider"
)

var logger = log.Log.WithName("cloudprovider-webhook")

// Args are the requirements to create a cloudprovider webhook.
type Args struct {
	Provider             string
	Mutator              extensionswebhook.Mutator
	EnableObjectSelector bool
}

// New creates a new cloudprovider webhook.
func New(mgr manager.Manager, args Args) (*extensionswebhook.Webhook, error) {
	logger := logger.WithValues("cloud-provider", args.Provider)

	types := []extensionswebhook.Type{{Obj: &corev1.Secret{}}}
	handler, err := extensionswebhook.NewBuilder(mgr, logger).WithMutator(args.Mutator, types...).Build()
	if err != nil {
		return nil, err
	}

	namespaceSelector := buildSelector(args.Provider)
	logger.Info("Creating webhook")

	webhook := &extensionswebhook.Webhook{
		Name:     WebhookName,
		Target:   extensionswebhook.TargetSeed,
		Provider: args.Provider,
		Types:    types,
		Webhook:  &admission.Webhook{Handler: handler, RecoverPanic: true},
		Path:     WebhookName,
		Selector: namespaceSelector,
	}

	if args.EnableObjectSelector {
		webhook.ObjectSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				v1beta1constants.GardenerPurpose: v1beta1constants.SecretNameCloudProvider,
			},
		}
	}

	return webhook, nil
}

func buildSelector(provider string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      v1beta1constants.LabelShootProvider,
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{provider},
			},
		},
	}
}
