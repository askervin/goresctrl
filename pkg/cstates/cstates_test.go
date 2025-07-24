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

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/intel/goresctrl/pkg/utils"
)

type mockFS struct {
	fPossibleCpus          func() (string, error)
	fCpuidleStates         func(cpu utils.ID) ([]int, error)
	fCpuidleStateAttrRead  func(cpu utils.ID, state int, attr string) (string, error)
	fCpuidleStateAttrWrite func(cpu utils.ID, state int, attr string, value string) error
}

func (fs *mockFS) PossibleCpus() (string, error) {
	return fs.fPossibleCpus()
}

func (fs *mockFS) CpuidleStates(cpu utils.ID) ([]int, error) {
	return fs.fCpuidleStates(cpu)
}

func (fs *mockFS) CpuidleStateAttrRead(cpu utils.ID, state int, attr string) (string, error) {
	return fs.fCpuidleStateAttrRead(cpu, state, attr)
}

func (fs *mockFS) CpuidleStateAttrWrite(cpu utils.ID, state int, attr string, value string) error {
	return fs.fCpuidleStateAttrWrite(cpu, state, attr, value)
}

func TestNewCstatesFromSysfs(t *testing.T) {
	// Make sure the platform supports cpuidle and disabling a C-state
	if _, err := os.Stat("/sys/devices/system/cpu/cpu0/cpuidle/state1/disable"); os.IsNotExist(err) {
		t.Skip("/sys/devices/system/cpu/cpu0/cpuidle/state1/disable does not exist")
	}
	possibleCpus, err := NewSysfs().PossibleCpus()
	if err != nil || possibleCpus == "" {
		t.Fatalf("Failed to get possible CPUs from sysfs: %v", err)
	}
	cpus, err := utils.NewIDSetFromString(possibleCpus)
	if err != nil {
		t.Fatalf("Failed to parse possible CPUs %q: %v", possibleCpus, err)
	}
	cpuCount := cpus.Size()

	// Read the disable attribute from all C-states of all CPUs
	cs, err := NewCstatesFromSysfs(FilterAttrs(DISABLE))
	if err != nil {
		t.Fatalf("NewCstatesFromSysfs failed: %v", err)
	}

	// compare read CPUs to the number of possible CPUs
	if cs.CPUs().Size() != cpuCount {
		t.Fatalf("Expected %d CPUs, got %d", cpuCount, cs.CPUs().Size())
	}
}

func TestPopulateFromFS(t *testing.T) {
	// This test runs in environments without access to cpuidle in
	// sysfs (like in virtual machines).
	cs := NewCstates()
	fs := &mockFS{
		fPossibleCpus: func() (string, error) {
			return "0-7", nil
		},
		fCpuidleStates: func(cpu utils.ID) ([]int, error) {
			return []int{0, 1, 2, 3, 4}, nil
		},
		fCpuidleStateAttrRead: func(cpu utils.ID, state int, attr string) (string, error) {
			// Fail if any other file will be read than filtered ones
			if cpu < 2 || cpu > 5 {
				t.Fatalf("Unexpected CPU ID %d", cpu)
			}
			if attr != "name" && (state != 1 && state != 2 && state != 4) {
				t.Fatalf("Unexpected read on cpu%d state%d attr %q", cpu, state, attr)
			}
			switch attr {
			case "name":
				return fmt.Sprintf("C%d", state), nil
			case "disable":
				if cpu == 3 && state > 0 {
					return "1", nil
				}
				return "0", nil
			case "time":
				return fmt.Sprintf("100%d%d", cpu, state), nil
			default:
				t.Fatalf("Unexpected read of attribute %q", attr)
			}
			return "", nil
		},
		fCpuidleStateAttrWrite: func(cpu utils.ID, state int, attr string, value string) error {
			t.Fatalf("Unexpected write of attribute %q", attr)
			return nil
		},
	}
	cs.fs = fs
	if err := cs.populateFromFS(FilterCPUs(2, 3, 4, 5), FilterNames("C1", "C2", "C4"), FilterAttrs(DISABLE, TIME)); err != nil {
		t.Fatalf("Failed to populate C-states from sysfs: %v", err)
	}

	// compare read CPUs to the number of possible CPUs
	if !slices.Equal(cs.CPUs().SortedMembers(), []utils.ID{2, 3, 4, 5}) {
		t.Fatalf("Expected CPUs [2 3 4 5], got %v", cs.CPUs().SortedMembers())
	}

	if !slices.Equal(cs.Names(), []string{"C1", "C2", "C4"}) {
		t.Fatalf("Expected C-state names [C0 C1 C2], got %v", cs.Names())
	}

	if !slices.Equal(cs.Attrs(), []AttrID{DISABLE, TIME}) {
		t.Fatalf("Expected attributes [disable time], got %v", cs.Attrs())
	}

	if cpu2c1time := cs.GetAttr(2, "C1", TIME); cpu2c1time == nil || *cpu2c1time != "10021" {
		t.Fatalf("Expected to find C1 time 10021 on CPU2, got %v", cpu2c1time)
	}

	if cpu5c4time := cs.GetAttr(5, "C4", TIME); cpu5c4time == nil || *cpu5c4time != "10054" {
		t.Fatalf("Expected to find C4 time 10054 on CPU4, got %v", cpu5c4time)
	}

	// Find which CPUs have C2, C3 (not even present) or C4 disabled.
	c2Disabled := cs.Copy(FilterNames("C2", "C3", "C4"), FilterAttrValues(DISABLE, "1"))
	if !slices.Equal(c2Disabled.CPUs().Members(), []utils.ID{3}) {
		t.Fatalf("Expected only CPU3 to have C2 disabled, got %v", c2Disabled.CPUs().SortedMembers())
	}

	// Enable C2 on CPU3.
	cpu3C2disable := "x"
	csFiltered := cs.Copy(FilterCPUs(3), FilterNames("C2"))
	// Override the fs write function to capture expected write, fail on all other writes.
	fs.fCpuidleStateAttrWrite = func(cpu utils.ID, state int, attr string, value string) error {
		if cpu == 3 && state == 2 && attr == "disable" {
			cpu3C2disable = value
			return nil
		}
		t.Fatalf("Unexpected write: cpu%d state%d attr: %q value: %q", cpu, state, attr, value)
		return nil
	}
	csFiltered.SetAttrs(DISABLE, "0")
	if err := csFiltered.Apply(); err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
	if cpu3C2disable != "0" {
		t.Fatalf("Expected to write '0' to cpu3 state2 disable, got %q", cpu3C2disable)
	}
	csFiltered.SetAttrs(DISABLE, "1")
	if cpu3C2disable != "0" {
		t.Fatalf("Expected to keep '0' in cpu3 state2 disable until Apply(), got %q", cpu3C2disable)
	}
	if err := csFiltered.Apply(); err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
	if cpu3C2disable != "1" {
		t.Fatalf("Expected to write '1' to cpu3 state2 disable, got %q", cpu3C2disable)
	}

	// Test that after clearing "disable" attributes Apply() will not write anything.
	// Force fail on every write.
	fs.fCpuidleStateAttrWrite = func(cpu utils.ID, state int, attr string, value string) error {
		t.Fatalf("Unexpected write: cpu%d state%d attr: %q value: %q", cpu, state, attr, value)
		return nil
	}
	csFiltered.ClearAttrs(DISABLE)
	if err := csFiltered.Apply(); err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}

	// output the C-states with String()
	csStr := cs.String()
	if !strings.Contains(csStr, "time=10021") {
		t.Fatalf("Expected to find time=10021 in C-states String(), got %q", csStr)
	}
	if csStr == "" {
		t.Fatalf("Expected non-empty C-states String(), got empty string")
	}
	csFilteredStr := csFiltered.String()
	t.Logf("cs=%s", csStr)
	t.Logf("csFiltered=%s", csFilteredStr)

}
