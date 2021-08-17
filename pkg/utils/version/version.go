// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package version

import (
	"fmt"
	"strings"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/Masterminds/semver"
)

var (
	// VersionConstraintK8sLessEqual115 is a version constraint for versions <= 1.15.
	VersionConstraintK8sLessEqual115 *semver.Constraints
	// VersionConstraintK8sGreaterEqual116 is a version constraint for versions >= 1.16.
	VersionConstraintK8sGreaterEqual116 *semver.Constraints
	// VersionConstraintK8sGreaterEqual117 is a version constraint for versions >= 1.17.
	VersionConstraintK8sGreaterEqual117 *semver.Constraints
	// VersionConstraintK8sGreaterEqual118 is a version constraint for versions >= 1.18.
	VersionConstraintK8sGreaterEqual118 *semver.Constraints
	// VersionConstraintK8sEqual118 is a version constraint for versions == 1.18.
	VersionConstraintK8sEqual118 *semver.Constraints
	// VersionConstraintK8sGreaterEqual119 is a version constraint for versions >= 1.19.
	VersionConstraintK8sGreaterEqual119 *semver.Constraints
	// VersionConstraintK8sLess119 is a version constraint for versions < 1.19.
	VersionConstraintK8sLess119 *semver.Constraints
	// VersionConstraintK8sEqual119 is a version constraint for versions == 1.19.
	VersionConstraintK8sEqual119 *semver.Constraints
	// VersionConstraintK8sGreaterEqual120 is a version constraint for versions >= 1.20.
	VersionConstraintK8sGreaterEqual120 *semver.Constraints
	// VersionConstraintK8sEqual120 is a version constraint for versions == 1.20.
	VersionConstraintK8sEqual120 *semver.Constraints
	// VersionConstraintK8sLessEqual121 is a version constraint for versions <= 1.21.
	VersionConstraintK8sLessEqual121 *semver.Constraints
)

func init() {
	var err error

	VersionConstraintK8sLessEqual115, err = semver.NewConstraint("<= 1.15.x")
	utilruntime.Must(err)
	VersionConstraintK8sGreaterEqual116, err = semver.NewConstraint(">= 1.16")
	utilruntime.Must(err)
	VersionConstraintK8sGreaterEqual117, err = semver.NewConstraint(">= 1.17")
	utilruntime.Must(err)
	VersionConstraintK8sGreaterEqual118, err = semver.NewConstraint(">= 1.18")
	utilruntime.Must(err)
	VersionConstraintK8sEqual118, err = semver.NewConstraint("1.18.x")
	utilruntime.Must(err)
	VersionConstraintK8sGreaterEqual119, err = semver.NewConstraint(">= 1.19")
	utilruntime.Must(err)
	VersionConstraintK8sEqual119, err = semver.NewConstraint("1.19.x")
	utilruntime.Must(err)
	VersionConstraintK8sLess119, err = semver.NewConstraint("< 1.19")
	utilruntime.Must(err)
	VersionConstraintK8sGreaterEqual120, err = semver.NewConstraint(">= 1.20")
	utilruntime.Must(err)
	VersionConstraintK8sEqual120, err = semver.NewConstraint("1.20.x")
	utilruntime.Must(err)
	VersionConstraintK8sLessEqual121, err = semver.NewConstraint("<= 1.21.x")
	utilruntime.Must(err)
}

// CompareVersions returns true if the constraint <version1> compared by <operator> to <version2>
// returns true, and false otherwise.
// The comparison is based on semantic versions, i.e. <version1> and <version2> will be converted
// if needed.
func CompareVersions(version1, operator, version2 string) (bool, error) {
	var (
		v1 = normalizeVersion(version1)
		v2 = normalizeVersion(version2)
	)

	return CheckVersionMeetsConstraint(v1, fmt.Sprintf("%s %s", operator, v2))
}

func normalizeVersion(version string) string {
	v := strings.Replace(version, "v", "", -1)
	idx := strings.IndexAny(v, "-+")
	if idx != -1 {
		v = v[:idx]
	}
	return v
}

// CheckVersionMeetsConstraint returns true if the <version> meets the <constraint>.
func CheckVersionMeetsConstraint(version, constraint string) (bool, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, err
	}

	v, err := semver.NewVersion(normalizeVersion(version))
	if err != nil {
		return false, err
	}

	return c.Check(v), nil
}
