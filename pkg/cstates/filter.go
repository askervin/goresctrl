/*
Copyright 2025 Intel Corporation

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

package cstates

import "github.com/intel/goresctrl/pkg/utils"

type CstatesFilter func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool

func FilterCPUs(ids ...utils.ID) CstatesFilter {
	idset := utils.NewIDSet(ids...)
	return func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool {
		return idset.Has(cpuid)
	}
}

func FilterAttrs(attributes ...AttrID) CstatesFilter {
	attrs := make(map[AttrID]struct{}, len(attributes)+1)
	for _, attr := range attributes {
		attrs[attr] = struct{}{}
	}
	return func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool {
		if attr == -1 {
			return true
		}
		_, ok := attrs[attr]
		return ok
	}
}

func FilterNames(cstateNames ...string) CstatesFilter {
	names := make(map[string]struct{}, len(cstateNames)+1)
	names[""] = struct{}{}
	for _, name := range cstateNames {
		names[name] = struct{}{}
	}
	return func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool {
		_, ok := names[cstateName]
		return ok
	}
}

func FilterAttrValues(attribute AttrID, values ...string) CstatesFilter {
	vals := make(map[string]struct{}, len(values))
	for _, v := range values {
		vals[v] = struct{}{}
	}
	return func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool {
		if attr == -1 || val == nil {
			return true
		}
		_, ok := vals[*val]
		return attr == attribute && ok
	}
}

func FilterAll(filters ...CstatesFilter) CstatesFilter {
	return func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool {
		for _, f := range filters {
			if !f(cpuid, cstateName, attr, val) {
				return false
			}
		}
		return true
	}
}

func FilterAny(filters ...CstatesFilter) CstatesFilter {
	return func(cpuid utils.ID, cstateName string, attr AttrID, val *string) bool {
		for _, f := range filters {
			if f(cpuid, cstateName, attr, val) {
				return true
			}
		}
		return false
	}
}
