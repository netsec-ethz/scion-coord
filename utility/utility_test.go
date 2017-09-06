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

import "testing"

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
	inc uint32
}

func TestIPFunctions(t *testing.T) {
	ipComparisonTests := []ipComparisonTest{
		ipComparisonTest{"0.0.0.0", "0.0.0.1", -1},
		ipComparisonTest{"0.0.0.0", "0.0.0.0", 0},
		ipComparisonTest{"0.0.0.1", "0.0.0.0", 1},
		ipComparisonTest{"0.1.0.0", "0.0.0.0", 1},
	}

	for _, ipComp := range ipComparisonTests {
		if IPCompare(ipComp.ip1, ipComp.ip2) != ipComp.comp || IPCompare(ipComp.ip2, ipComp.ip1) !=
			-ipComp.comp {
			t.Errorf("IP comparison failed for %v and %v", ipComp.ip1, ipComp.ip2)
		}
	}

	ipConvertTests := []ipConvertTest{
		ipConvertTest{"0.0.0.0", 0},
		ipConvertTest{"0.0.0.1", 1},
		ipConvertTest{"0.0.1.0", 256},
		ipConvertTest{"0.1.0.0", 65536},
		ipConvertTest{"1.0.0.0", 16777216},
	}

	for _, ipConv := range ipConvertTests {
		if IPToInt(ipConv.ipStr) != ipConv.ipInt || IntToIP(ipConv.ipInt) != ipConv.ipStr {
			t.Errorf("IP conversion failed for %v", ipConv.ipStr)
		}
	}

	ipIncrementTests := []ipIncrementTest{
		ipIncrementTest{"192.168.1.1", "192.168.1.3", 2},
		ipIncrementTest{"255.255.255.255", "0.0.0.0", 1},
	}

	for _, ipInc := range ipIncrementTests {
		if IPIncrement(ipInc.ip1, ipInc.inc) != ipInc.ip2 || IPIncrement(ipInc.ip2, -ipInc.inc) !=
			ipInc.ip1 {
			t.Errorf("IP increment failed when incrementing %v by %v", ipInc.ip1, ipInc.inc)
		}
	}

}
