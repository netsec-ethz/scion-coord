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
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

const (
	// ScionLabISDOffsetAddr : the first scionlab ISD (not taking backbone into account)
	ScionLabISDOffsetAddr = 17
	// ScionLabInfrastructureASOffsetAddr : infrastructure addresses start here
	ScionLabInfrastructureASOffsetAddr = 0xFFAA00000001
	// ScionlabUserASOffsetAddr : user ASes addresses start here:
	ScionlabUserASOffsetAddr = 0xFFAA00010001
)

// Simple utility function to copy a file.
func CopyFile(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, sourcefile)
	if err == nil {
		sourceInfo, _ := os.Stat(source)
		// TODO(jonghoonkwon): do proper error logging!
		err = os.Chmod(dest, sourceInfo.Mode())
	}
	return
}

// CopyPath copies the src path to dst. Example: CopyPath("/tmp/a", "/tmp/b") will copy /tmp/a/* to /tmp/b/
// replacing contents if files or dirs existed in b with the same names as a/*
func CopyPath(src, dst string) error {
	srcDir, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open directory %v: %v", src, err)
	}
	objects, err := srcDir.Readdir(-1)
	if err != nil {
		return fmt.Errorf("error listing directory %v: %v", src, err)
	}
	for _, obj := range objects {
		if obj.IsDir() {
			subDirSrc := filepath.Join(src, obj.Name())
			subDirDst := filepath.Join(dst, obj.Name())
			srcStat, err := os.Stat(subDirSrc)
			if err != nil {
				return fmt.Errorf("error stating %v: %v", subDirSrc, err)
			}
			if err := os.Mkdir(subDirDst, srcStat.Mode()); err != nil {
				return fmt.Errorf("error creating directory %v: %v", subDirDst, err)
			}
			if err := CopyPath(subDirSrc, subDirDst); err != nil {
				return err
			}
		} else {
			if err := CopyFile(filepath.Join(src, obj.Name()), filepath.Join(dst, obj.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// Some helper functions for IP addresses
func IPToInt(ip string) uint32 {
	return binary.BigEndian.Uint32(net.ParseIP(ip)[12:])
}

func IntToIP(ipInt uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ipInt>>24), byte(ipInt>>16), byte(ipInt>>8), byte(ipInt))
}

func IPIncrement(ip string, diff int32) string {
	temp := IPToInt(ip)
	if diff > 0 {
		temp += uint32(diff)
	} else {
		temp -= uint32(-diff)
	}
	return IntToIP(temp)
}

// Returns -1, if ip1 < ip2, 0, if ip1 == ip2, +1, if ip1 > ip2
func IPCompare(ip1, ip2 string) int8 {
	if diff := int(IPToInt(ip1)) - int(IPToInt(ip2)); diff > 0 {
		return 1
	} else if diff == 0 {
		return 0
	} else {
		return -1
	}
}

// Create IA string from ISD and AS IDs
func IAString(isd, as interface{}) string {
	return fmt.Sprintf("%v-%v", isd, as)
}

// Parses a BR name and returns the BRID
func BRIDFromString(s string) (uint16, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid BR name structure: %v", s)
	}
	id, err := strconv.ParseInt(parts[2], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("unable to parse BRID: %v", err)
	}
	return uint16(id), nil
}

// Returns the smallest integer in the range [min, max] that is not present in the given ids
func GetAvailableID(ids []int, min, max int) (int, error) {
	res := min
	sort.Ints(ids)
	for _, x := range ids {
		if res < x {
			break
		}
		res = x + 1
	}
	if res > max {
		return 0, errors.New("no free ID found")
	}
	return res, nil
}

// general helper function which fills a template with given data and saves it
// to the specified path
func FillTemplateAndSave(templatePath string, data interface{}, savePath string) error {
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing template %v: %v", templatePath, err)
	}
	f, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("error creating file %v: %v", savePath, err)
	}
	err = t.Execute(f, data)
	f.Close()

	if err != nil {
		return fmt.Errorf("error executing template file %v: %v", templatePath, err)
	}
	return nil
}

// SendJSON will marshall the passed object and write it to the ResponseWriter in the argument.
func SendJSON(object interface{}, w http.ResponseWriter) error {
	b, err := json.Marshal(object)
	if err == nil {
		_, err = fmt.Fprintln(w, string(b))
	}
	return err
}

// SendJSONError sends the object as JSON, with a 400 HTTP error code
func SendJSONError(object interface{}, w http.ResponseWriter) error {
	b, err := json.Marshal(object)
	if err == nil {
		http.Error(w, string(b), http.StatusBadRequest)
	}
	return err
}

// MapOldIAToNewOne returns a valid SCION IA address given
// the old one. E.g. (1,1001) -> (17, ffaa:0001:0000)
// The convention for scionlab user ASes is to start from exactly ffaa:0001:0000, meaning
// we reserve ffaa:0000:0000 to ffaa:0000:ffff for infrastructure.
func MapOldIAToNewOne(ISDid, ASid uint) (uint, uint) {
	if ISDid < 1 || ISDid > 8 || ASid < 1001 || ASid > 2000 {
		// invalid request for scionlab
		return 0, 0
	}

	return ISDid + ScionLabISDOffsetAddr - 1, ASid - 1001 + ScionlabUserASOffsetAddr
}
