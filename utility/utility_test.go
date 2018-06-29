// Copyright 2017 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utility

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
)

type ipComparisonTest struct {
	ip1  string
	ip2  string
	comp int8
}

type ipConvertTest struct {
	ipStr string
	ipInt uint32
}

type ipIncrementTest struct {
	ip1 string
	ip2 string
	inc int32
}

type brTest struct {
	name  string
	error bool
	id    uint16
}

func TestIPFunctions(t *testing.T) {
	ipComparisonTests := []ipComparisonTest{
		{"0.0.0.0", "0.0.0.1", -1},
		{"0.0.0.0", "0.0.0.0", 0},
		{"0.0.0.1", "0.0.0.0", 1},
		{"0.1.0.0", "0.0.0.0", 1},
	}

	for _, ipComp := range ipComparisonTests {
		if IPCompare(ipComp.ip1, ipComp.ip2) != ipComp.comp || IPCompare(ipComp.ip2, ipComp.ip1) !=
			-ipComp.comp {
			t.Errorf("IP comparison failed for %v and %v", ipComp.ip1, ipComp.ip2)
		}
	}

	ipConvertTests := []ipConvertTest{
		{"0.0.0.0", 0},
		{"0.0.0.1", 1},
		{"0.0.1.0", 256},
		{"0.1.0.0", 65536},
		{"1.0.0.0", 16777216},
	}

	for _, ipConv := range ipConvertTests {
		if IPToInt(ipConv.ipStr) != ipConv.ipInt || IntToIP(ipConv.ipInt) != ipConv.ipStr {
			t.Errorf("IP conversion failed for %v", ipConv.ipStr)
		}
	}

	ipIncrementTests := []ipIncrementTest{
		{"192.168.1.1", "192.168.1.3", 2},
		{"255.255.255.255", "0.0.0.0", 1},
	}

	for _, ipInc := range ipIncrementTests {
		if IPIncrement(ipInc.ip1, ipInc.inc) != ipInc.ip2 || IPIncrement(ipInc.ip2, -ipInc.inc) !=
			ipInc.ip1 {
			t.Errorf("IP increment failed when incrementing %v by %v", ipInc.ip1, ipInc.inc)
		}
	}

}

func TestBRIDFromString(t *testing.T) {
	brTests := []brTest{
		{"br1-2-1", false, 1},
		{"br123", true, 0}, // conversion should fail and return 0
		{"br3-4-0", false, 0},
	}

	for _, brT := range brTests {
		id, err := BRIDFromString(brT.name)
		if id != brT.id || (brT.error == (err == nil)) {
			t.Errorf("Conversion of BR string failed for %v", brT.name)
		}
	}
}

var getAvailableIDtests = []struct {
	min      int
	max      int
	used     []int
	expected int
	err      bool
}{
	{0, 10, []int{0}, 1, false},
	{0, 10, []int{1}, 0, false},
	{0, 10, []int{}, 0, false},
	{0, 10, []int{0, 1}, 2, false},
	{0, 10, []int{0, 1, 2}, 3, false},
	{0, 10, []int{0, 2}, 1, false},
	{0, 10, []int{0, 1, 6}, 2, false},
	{0, 0, []int{}, 0, false},
	{0, 0, []int{0}, 0, true},
	{1, 0, []int{}, 0, true},
}

func TestGetAvailableID(t *testing.T) {
	for i, tt := range getAvailableIDtests {
		actual, err := GetAvailableID(tt.used, tt.min, tt.max)
		if (err != nil) != tt.err {
			t.Errorf("Expected error? %v, but error is: %v", tt.err, err)
			t.Errorf("Test table index %d, content:\n%v", i, tt)
		}
		if actual != tt.expected {
			t.Errorf("Expected %v, got %v", tt.expected, actual)
			t.Errorf("Test table index %d, content:\n%v", i, tt)
		}
	}
}

func TestCopyFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := ioutil.WriteFile(src, []byte("test string\n"), 0666); err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := CopyFile(src, dst); err != nil {
		t.Errorf("Error: %v", err)
	}
	contents, err := ioutil.ReadFile(dst)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !bytes.Equal(contents, []byte("test string\n")) {
		t.Errorf("Test file contents differ on %v and %v", src, dst)
	}
}

func TestCopyPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "utility_ut_")
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	defer os.RemoveAll(dir)
	// temp/src/a
	// temp/src/subdir/b
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.Mkdir(src, 0777); err != nil {
		t.Errorf("Error: %v", err)
	}
	if _, err := os.Create(filepath.Join(src, "a")); err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(src, "a"), []byte("test a\n"), 0666); err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := os.Mkdir(filepath.Join(src, "subdir"), 0777); err != nil {
		t.Errorf("Error: %v", err)
	}
	if _, err := os.Create(filepath.Join(src, "subdir", "b")); err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(src, "subdir", "b"), []byte("test b\n"), 0666); err != nil {
		t.Errorf("Error: %v", err)
	}

	if err := os.Mkdir(dst, 0777); err != nil {
		t.Errorf("Error: %v", err)
	}
	if err := CopyPath(src, dst); err != nil {
		t.Errorf("Error: %v", err)
	}

	content, err := ioutil.ReadFile(filepath.Join(dst, "a"))
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !bytes.Equal(content, []byte("test a\n")) {
		t.Errorf("Test file \"a\" contents differ on %v and %v", src, dst)
	}
	content, err = ioutil.ReadFile(filepath.Join(dst, "subdir", "b"))
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	if !bytes.Equal(content, []byte("test b\n")) {
		t.Errorf("Test file \"b\" contents differ on %v and %v",
			filepath.Join(src, "subdir"), filepath.Join(dst, "subdir"))
	}
}

var mapOldIAToNewOneTests = []struct {
	fromISD addr.ISD
	fromAS  addr.AS
	toISD   addr.ISD
	toAS    addr.AS
}{
	{20, 1001, 60, 0xFFAA00010001},
	{1, 1001, 17, 0xFFAA00010001},
	{2, 1001, 18, 0xFFAA00010001},
	{0, 1001, 0, 0},
	{42, 1001, 0, 0},
	{1, 1000, 0, 0},
	{1, 2001, 17, 0xFFAA000103E9}, // scionbox valid
	{8, 1017, 24, 0xFFAA00010011},
}

func TestMapOldIAToNewOne(t *testing.T) {
	for index, c := range mapOldIAToNewOneTests {
		IA := MapOldIAToNewOne(c.fromISD, c.fromAS)
		if IA.I != c.toISD || IA.A != c.toAS {
			t.Errorf("FAILED mapping #%d (%v,%v) -> (%v,%v). Should be (%v,%v)", index, c.fromISD, c.fromAS, IA.I, IA.A, c.toISD, c.toAS)
		}
	}
	// t.Errorf("we are done")
}
