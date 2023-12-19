// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package extensionlabels_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apiserver/features"
)

func TestExtensionLabels(t *testing.T) {
	features.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "AdmissionPlugin Global ExtensionLabels Suite")
}
