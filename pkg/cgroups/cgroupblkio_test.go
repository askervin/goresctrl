// Copyright 2020 Intel Corporation. All Rights Reserved.
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

package cgroups

import (
	"bytes"
	"io"
	"testing"

	"github.com/intel/goresctrl/pkg/testutils"
)

func TestUpdateAppend(t *testing.T) {
	tcases := []struct {
		name                    string
		inputMajMinVals         [][]int64
		inputItem               []int64
		expectedMajMinVal       [][]int64
		expectedErrorCount      int
		expectedErrorSubstrings []string
	}{
		{
			name:              "update empty list",
			inputItem:         []int64{1, 2, 3},
			expectedMajMinVal: [][]int64{{1, 2, 3}},
		},
		{
			name:              "update appends non-existing element",
			inputMajMinVals:   [][]int64{{10, 20, 30}, {40, 50, 60}},
			inputItem:         []int64{1, 2, 3},
			expectedMajMinVal: [][]int64{{10, 20, 30}, {40, 50, 60}, {1, 2, 3}},
		},
		{
			name:              "update the first existing element",
			inputMajMinVals:   [][]int64{{10, 20, 30}, {40, 50, 60}, {40, 50, 60}},
			inputItem:         []int64{40, 50, 66},
			expectedMajMinVal: [][]int64{{10, 20, 30}, {40, 50, 66}, {40, 50, 60}},
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			devWeights := OciDeviceWeights{}
			devRates := OciDeviceRates{}
			expDevWeights := OciDeviceWeights{}
			expDevRates := OciDeviceRates{}
			for _, item := range tc.inputMajMinVals {
				devWeights.Append(item[0], item[1], item[2])
				devRates.Append(item[0], item[1], item[2])
			}
			devWeights.Update(tc.inputItem[0], tc.inputItem[1], tc.inputItem[2])
			devRates.Update(tc.inputItem[0], tc.inputItem[1], tc.inputItem[2])
			for _, item := range tc.expectedMajMinVal {
				expDevWeights = append(expDevWeights, OciDeviceWeight{item[0], item[1], item[2]})
				expDevRates = append(expDevRates, OciDeviceRate{item[0], item[1], item[2]})
			}
			testutils.VerifyDeepEqual(t, "device weights", expDevWeights, devWeights)
			testutils.VerifyDeepEqual(t, "device rates", expDevRates, devRates)
		})
	}
}

var fsBlkioUtFiles map[string]mockFile = map[string]mockFile{
	mountDir + "/blkio/mockpods/clean/blkio.bfq.weight":                 {data: []byte("100\n")},
	mountDir + "/blkio/mockpods/clean/blkio.bfq.weight_device":          {},
	mountDir + "/blkio/mockpods/clean/blkio.throttle.read_bps_device":   {},
	mountDir + "/blkio/mockpods/clean/blkio.throttle.write_bps_device":  {},
	mountDir + "/blkio/mockpods/clean/blkio.throttle.read_iops_device":  {},
	mountDir + "/blkio/mockpods/clean/blkio.throttle.write_iops_device": {},

	mountDir + "/blkio/mockpods/reset/blkio.bfq.weight":                 {data: []byte("200\n")},
	mountDir + "/blkio/mockpods/reset/blkio.bfq.weight_device":          {data: []byte("default 200\n1:2 3\n4:5 6\n")},
	mountDir + "/blkio/mockpods/reset/blkio.throttle.read_bps_device":   {data: []byte("11:12 13\n14:15 16\n")},
	mountDir + "/blkio/mockpods/reset/blkio.throttle.write_bps_device":  {data: []byte("21:22 23\n")},
	mountDir + "/blkio/mockpods/reset/blkio.throttle.read_iops_device":  {data: []byte("31:32 33\n")},
	mountDir + "/blkio/mockpods/reset/blkio.throttle.write_iops_device": {data: []byte("41:42 43\n")},

	mountDir + "/blkio/mockpods/merge/blkio.bfq.weight":                 {data: []byte("200\n")},
	mountDir + "/blkio/mockpods/merge/blkio.bfq.weight_device":          {data: []byte("default 200\n1:2 3\n4:5 6\n7:8 9")},
	mountDir + "/blkio/mockpods/merge/blkio.throttle.read_bps_device":   {data: []byte("11:12 13\n14:15 16\n")},
	mountDir + "/blkio/mockpods/merge/blkio.throttle.write_bps_device":  {data: []byte("21:22 23\n24:25 26\n")},
	mountDir + "/blkio/mockpods/merge/blkio.throttle.read_iops_device":  {data: []byte("31:32 33\n331:332 333\n")},
	mountDir + "/blkio/mockpods/merge/blkio.throttle.write_iops_device": {data: []byte("41:42 43\n441:442 443\n")},

	// parseok:
	// test weight without linefeed
	// test weight_device file with real "default" line
	// test parsing two lines and skipping empty lines
	// test single line file
	// test single line, missing LF at the end
	// test small and large values
	mountDir + "/blkio/parseok/blkio.bfq.weight":                 {data: []byte("1")},
	mountDir + "/blkio/parseok/blkio.bfq.weight_device":          {data: []byte("default 10\n1:2 3\n")},
	mountDir + "/blkio/parseok/blkio.throttle.read_bps_device":   {data: []byte("\n11:22 33\n\n111:222 333\n")},
	mountDir + "/blkio/parseok/blkio.throttle.write_bps_device":  {data: []byte("1111:2222 3333\n")},
	mountDir + "/blkio/parseok/blkio.throttle.read_iops_device":  {data: []byte("11111:22222 33333")},
	mountDir + "/blkio/parseok/blkio.throttle.write_iops_device": {data: []byte("0:0 0\n4294967296:4294967297 9223372036854775807\n")},

	// parse-err:
	// weight: not a number
	// weight_device: test bad line in the middle
	// read_bps_device: test no spaces
	// write_bps_device: test too many spaces
	// read_iobps_device: test no colons
	// write_iobps_device: test missing number
	mountDir + "/blkio/parse-err/blkio.bfq.weight":                 {data: []byte("xyz")},
	mountDir + "/blkio/parse-err/blkio.bfq.weight_device":          {data: []byte("default 10\n1:2 3\nbad\n4:5 6\n")},
	mountDir + "/blkio/parse-err/blkio.throttle.read_bps_device":   {data: []byte("11:22:33")},
	mountDir + "/blkio/parse-err/blkio.throttle.write_bps_device":  {data: []byte("1111 2222 3333 \n")},
	mountDir + "/blkio/parse-err/blkio.throttle.read_iops_device":  {data: []byte("1111122222 33333")},
	mountDir + "/blkio/parse-err/blkio.throttle.write_iops_device": {data: []byte("0: 0\n")},
}

// TestResetBlkioParameters: unit test for ResetBlkioParameters()
func TestResetBlkioParameters(t *testing.T) {
	tcases := []struct {
		name                    string
		fsi                     fsiIface
		cntnrDir                string
		blockIO                 OciBlockIOParameters
		expectedFsWrites        map[string][][]byte
		expectedBlockIO         *OciBlockIOParameters
		expectedErrorCount      int
		expectedErrorSubstrings []string
	}{
		{
			name:     "write to clean cgroups",
			fsi:      NewFsiMock(fsBlkioUtFiles),
			cntnrDir: "mockpods/clean",
			blockIO: OciBlockIOParameters{
				Weight:                  222,
				WeightDevice:            OciDeviceWeights{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}},
				ThrottleReadBpsDevice:   OciDeviceRates{{11, 12, 13}, {111, 112, 113}},
				ThrottleWriteBpsDevice:  OciDeviceRates{{21, 22, 23}, {221, 222, 223}},
				ThrottleReadIOPSDevice:  OciDeviceRates{{31, 32, 33}, {331, 332, 333}},
				ThrottleWriteIOPSDevice: OciDeviceRates{{41, 42, 43}, {441, 442, 443}},
			},
			expectedFsWrites: map[string][][]byte{
				mountDir + "/blkio/mockpods/clean/blkio.bfq.weight":                 {[]byte("222")},
				mountDir + "/blkio/mockpods/clean/blkio.bfq.weight_device":          {[]byte("1:2 3"), []byte("4:5 6"), []byte("7:8 9")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.read_bps_device":   {[]byte("11:12 13"), []byte("111:112 113")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.write_bps_device":  {[]byte("21:22 23"), []byte("221:222 223")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.read_iops_device":  {[]byte("31:32 33"), []byte("331:332 333")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.write_iops_device": {[]byte("41:42 43"), []byte("441:442 443")},
			},
		},
		{
			name:     "reset all existing",
			fsi:      NewFsiMock(fsBlkioUtFiles),
			cntnrDir: "mockpods/reset",
			blockIO:  NewOciBlockIOParameters(),
			expectedFsWrites: map[string][][]byte{
				mountDir + "/blkio/mockpods/reset/blkio.bfq.weight_device":          {[]byte("1:2 0"), []byte("4:5 0")},
				mountDir + "/blkio/mockpods/reset/blkio.throttle.read_bps_device":   {[]byte("11:12 0"), []byte("14:15 0")},
				mountDir + "/blkio/mockpods/reset/blkio.throttle.write_bps_device":  {[]byte("21:22 0")},
				mountDir + "/blkio/mockpods/reset/blkio.throttle.read_iops_device":  {[]byte("31:32 0")},
				mountDir + "/blkio/mockpods/reset/blkio.throttle.write_iops_device": {[]byte("41:42 0")},
			},
		},
		{
			name:     "merge",
			fsi:      NewFsiMock(fsBlkioUtFiles),
			cntnrDir: "mockpods/merge",
			blockIO: OciBlockIOParameters{
				Weight:                  80,
				WeightDevice:            OciDeviceWeights{{1, 2, 1113}, {7, 8, 9}},       // drop middle, update first, keep last
				ThrottleReadBpsDevice:   OciDeviceRates{{11, 12, 13}},                    // keep the first entry
				ThrottleWriteBpsDevice:  OciDeviceRates{{24, 25, 26}},                    // keep the last entry
				ThrottleReadIOPSDevice:  OciDeviceRates{{31, 32, 33}, {331, 332, 333}},   // keep all
				ThrottleWriteIOPSDevice: OciDeviceRates{{41, 42, 430}, {441, 442, 4430}}, // change all
			},
			expectedFsWrites: map[string][][]byte{
				mountDir + "/blkio/mockpods/merge/blkio.bfq.weight":                 {[]byte("80")},
				mountDir + "/blkio/mockpods/merge/blkio.bfq.weight_device":          {[]byte("1:2 1113"), []byte("7:8 9"), []byte("4:5 0")},
				mountDir + "/blkio/mockpods/merge/blkio.throttle.read_bps_device":   {[]byte("11:12 13"), []byte("14:15 0")},
				mountDir + "/blkio/mockpods/merge/blkio.throttle.write_bps_device":  {[]byte("24:25 26"), []byte("21:22 0")},
				mountDir + "/blkio/mockpods/merge/blkio.throttle.read_iops_device":  {[]byte("31:32 33"), []byte("331:332 333")},
				mountDir + "/blkio/mockpods/merge/blkio.throttle.write_iops_device": {[]byte("41:42 430"), []byte("441:442 4430")},
			},
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			fsi = tc.fsi
			err := ResetBlkioParameters(tc.cntnrDir, tc.blockIO)
			testutils.VerifyError(t, err, tc.expectedErrorCount, tc.expectedErrorSubstrings)
			validateWriteHistory(t, tc.expectedFsWrites, fsi.(*fsMock).files)
		})
	}
}

func validateWriteHistory(t *testing.T, expected map[string][][]byte, filesystem map[string]*mockFile) {
	for expFilename, expWrites := range expected {
		mf, ok := filesystem[expFilename]
		if !ok {
			t.Errorf("expected writes to %q, but file is missing", expFilename)
			return
		}
		obsWrites := mf.writeHistory
		if len(expWrites) != len(obsWrites) {
			t.Errorf("expected writes to %q differ: expected %v, observed %v", expFilename, expWrites, obsWrites)
			return
		}
		for i, expWrite := range expWrites {
			if bytes.Compare(expWrite, obsWrites[i]) != 0 {
				t.Errorf("expected write at position %d differs: expected %v, observed %v", i, expWrites, obsWrites)
			}
		}
	}
}

// TestGetBlkioParameters: unit test for GetBlkioParameters()
func TestGetBlkioParameters(t *testing.T) {
	tcases := []struct {
		name                    string
		fsi                     fsiIface
		fsFuncs                 map[string]mockFile
		cntnrDir                string
		readsFail               int
		fsContent               map[string]string
		expectedBlockIO         *OciBlockIOParameters
		expectedErrorCount      int
		expectedErrorSubstrings []string
	}{
		{
			name: "all clean and empty",
			fsi:  NewFsiMock(fsBlkioUtFiles),
			fsFuncs: map[string]mockFile{
				// reuse clean directory, but force weight file empty
				mountDir + "/blkio/mockpods/clean/blkio.bfq.weight": {
					read: func([]byte) (int, error) { return 0, io.EOF },
				},
			},
			cntnrDir:                "mockpods/clean",
			expectedBlockIO:         &OciBlockIOParameters{Weight: -1},
			expectedErrorCount:      1, // weight is not expected to be empty
			expectedErrorSubstrings: []string{"parsing weight"},
		},
		{
			name:     "everything defined",
			fsi:      NewFsiMock(fsBlkioUtFiles),
			cntnrDir: "/parseok",
			expectedBlockIO: &OciBlockIOParameters{
				Weight:                  1,
				WeightDevice:            OciDeviceWeights{{1, 2, 3}},
				ThrottleReadBpsDevice:   OciDeviceRates{{11, 22, 33}, {111, 222, 333}},
				ThrottleWriteBpsDevice:  OciDeviceRates{{1111, 2222, 3333}},
				ThrottleReadIOPSDevice:  OciDeviceRates{{11111, 22222, 33333}},
				ThrottleWriteIOPSDevice: OciDeviceRates{{0, 0, 0}, {4294967296, 4294967297, 9223372036854775807}},
			},
		},
		{
			name:                    "test bad files",
			fsi:                     NewFsiMock(fsBlkioUtFiles),
			cntnrDir:                "/parse-err",
			expectedErrorCount:      6,
			expectedErrorSubstrings: []string{"bad", "xyz", "11:22:33", "1111 2222 3333 ", "1111122222 33333", "0: 0"},
			expectedBlockIO: &OciBlockIOParameters{
				Weight:       -1,
				WeightDevice: OciDeviceWeights{{1, 2, 3}, {4, 5, 6}},
			},
		},
		{
			name:               "all files missing",
			fsi:                NewFsiMock(fsBlkioUtFiles),
			cntnrDir:           "/this/container/does/not/exist",
			expectedBlockIO:    &OciBlockIOParameters{Weight: -1},
			expectedErrorCount: 6,
			expectedErrorSubstrings: []string{
				"file not found",
				"blkio.bfq.weight",
				"blkio.bfq.weight_device",
				"blkio.throttle.read_bps_device",
				"blkio.throttle.write_bps_device",
				"blkio.throttle.read_iops_device",
				"blkio.throttle.write_iops_device",
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			fsi = tc.fsi
			overrideFsFuncs(fsi.(*fsMock), tc.fsFuncs)
			blockIO, err := GetBlkioParameters(tc.cntnrDir)
			testutils.VerifyError(t, err, tc.expectedErrorCount, tc.expectedErrorSubstrings)
			if tc.expectedBlockIO != nil {
				testutils.VerifyDeepEqual(t, "blockio parameters", *tc.expectedBlockIO, blockIO)
			}
		})
	}
}

func overrideFsFuncs(fsm *fsMock, fsFuncs map[string]mockFile) {
	for filename, mf := range fsFuncs {
		if mf.open != nil {
			fsm.files[filename].open = mf.open
		}
		if mf.read != nil {
			fsm.files[filename].read = mf.read
		}
		if mf.write != nil {
			fsm.files[filename].write = mf.write
		}
	}
}

// TestSetBlkioParameters: unit test for SetBlkioParameters()
func TestSetBlkioParameters(t *testing.T) {
	tcases := []struct {
		name                    string
		fsi                     fsiIface
		fsFuncs                 map[string]mockFile
		cntnrDir                string
		blockIO                 OciBlockIOParameters
		writesFail              int
		expectedFsWrites        map[string][][]byte
		expectedErrorCount      int
		expectedErrorSubstrings []string
	}{
		{
			name:       "write full OCI struct",
			fsi:  NewFsiMock(fsBlkioUtFiles),
			cntnrDir: "/mockpods/clean",
			blockIO: OciBlockIOParameters{
				Weight:                  10,
				WeightDevice:            OciDeviceWeights{{Major: 1, Minor: 2, Weight: 3}},
				ThrottleReadBpsDevice:   OciDeviceRates{{Major: 11, Minor: 12, Rate: 13}},
				ThrottleWriteBpsDevice:  OciDeviceRates{{Major: 21, Minor: 22, Rate: 23}},
				ThrottleReadIOPSDevice:  OciDeviceRates{{Major: 31, Minor: 32, Rate: 33}},
				ThrottleWriteIOPSDevice: OciDeviceRates{{Major: 41, Minor: 42, Rate: 43}},
			},
			expectedFsWrites: map[string][][]byte{
				mountDir + "/blkio/mockpods/clean/blkio.bfq.weight":                 {[]byte("10")},
				mountDir + "/blkio/mockpods/clean/blkio.bfq.weight_device":          {[]byte("1:2 3")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.read_bps_device":   {[]byte("11:12 13")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.write_bps_device":  {[]byte("21:22 23")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.read_iops_device":  {[]byte("31:32 33")},
				mountDir + "/blkio/mockpods/clean/blkio.throttle.write_iops_device": {[]byte("41:42 43")},
			},
		},
		//		{
		// 			name:       "write empty struct",
		// 			cgroupsDir: "/my/empty",
		// 			blockIO:    OciBlockIOParameters{},
		// 			expectedFsWrites: map[string]string{
		// 				"/my/empty/blkio.bfq.weight": "0",
		// 			},
		// 		},
		// 		{
		// 			name:       "multidevice weight and throttling, no weight write on -1",
		// 			cgroupsDir: "/my/multidev",
		// 			blockIO: OciBlockIOParameters{
		// 				Weight:                  -1,
		// 				WeightDevice:            OciDeviceWeights{{1, 2, 3}, {4, 5, 6}},
		// 				ThrottleReadBpsDevice:   OciDeviceRates{{11, 12, 13}, {111, 112, 113}},
		// 				ThrottleWriteBpsDevice:  OciDeviceRates{{21, 22, 23}, {221, 222, 223}},
		// 				ThrottleReadIOPSDevice:  OciDeviceRates{{31, 32, 33}, {331, 332, 333}},
		// 				ThrottleWriteIOPSDevice: OciDeviceRates{{41, 42, 43}, {441, 442, 443}},
		// 			},
		// 			expectedFsWrites: map[string]string{
		// 				"/my/multidev/blkio.bfq.weight_device":          "1:2 3+4:5 6",
		// 				"/my/multidev/blkio.throttle.read_bps_device":   "11:12 13+111:112 113",
		// 				"/my/multidev/blkio.throttle.write_bps_device":  "21:22 23+221:222 223",
		// 				"/my/multidev/blkio.throttle.read_iops_device":  "31:32 33+331:332 333",
		// 				"/my/multidev/blkio.throttle.write_iops_device": "41:42 43+441:442 443",
		// 			},
		// 		},
		// 		{
		// 			name:             "no bfq.weight",
		// 			cgroupsDir:       "/my/nobfq",
		// 			blockIO:          OciBlockIOParameters{Weight: 100},
		// 			writesFail:       1,
		// 			expectedFsWrites: map[string]string{"/my/nobfq/blkio.weight": "100"},
		// 		},
		// 		{
		// 			name:       "all writes fail",
		// 			cgroupsDir: "/my/writesfail",
		// 			blockIO: OciBlockIOParameters{
		// 				Weight:       -1,
		// 				WeightDevice: OciDeviceWeights{{1, 0, 100}},
		// 			},
		// 			writesFail:         9999,
		// 			expectedErrorCount: 1,
		// 			expectedErrorSubstrings: []string{
		// 				"could not write content \"1:0 100\" to any of files",
		// 				"\"blkio.bfq.weight_device\"",
		// 				"\"blkio.weight_device\"",
		// 			},
		// 		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			fsi = tc.fsi
			overrideFsFuncs(fsi.(*fsMock), tc.fsFuncs)
			err := SetBlkioParameters(tc.cntnrDir, tc.blockIO)
			testutils.VerifyError(t, err, tc.expectedErrorCount, tc.expectedErrorSubstrings)
			validateWriteHistory(t, tc.expectedFsWrites, fsi.(*fsMock).files)
		})
	}
}

// // mockPlatform implements mock versions of platformInterface functions.
// type mockPlatform struct {
// 	fsOrigContent map[string]string
// 	fsWrites      map[string]string
// 	readsFail     int
// 	writesFail    int
// }

// func (mpf *mockPlatform) readFromFile(filename string) (string, error) {
// 	if mpf.readsFail > 0 {
// 		mpf.readsFail--
// 		return "", fmt.Errorf("mockPlatofrm: reading from %#v failed", filename)
// 	}
// 	if content, ok := mpf.fsOrigContent[filename]; ok {
// 		return content, nil
// 	}
// 	return "", fmt.Errorf("mockPlatform: file not found %#v", filename)
// }

// func (mpf *mockPlatform) writeToFile(filename string, content string) error {
// 	var newContent string
// 	if mpf.writesFail > 0 {
// 		mpf.writesFail--
// 		return fmt.Errorf("mockPlatform: writing to %#v failed", filename)
// 	}
// 	if oldContent, ok := mpf.fsWrites[filename]; ok {
// 		newContent = fmt.Sprintf("%s+%s", oldContent, content)
// 	} else {
// 		newContent = content
// 	}
// 	mpf.fsWrites[filename] = newContent
// 	return nil
// }
