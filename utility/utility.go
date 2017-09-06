package utility

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

// Simple utility function to copy a file.
func CopyFile(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destfile.Close()
	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		sourceinfo, _ := os.Stat(source)
		// TODO (jonghoonkwon): do proper error logging!
		err = os.Chmod(dest, sourceinfo.Mode())
	}
	return
}

// Some helper functions for IP addresses
func IPToInt(ip string) uint32 {
	return binary.BigEndian.Uint32(net.ParseIP(ip))
}

func IntToIP(ipInt uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ipInt>>24), byte(ipInt>>16), byte(ipInt>>8), byte(ipInt))
}

func IPIncrement(ip string, diff uint32) string {
	return IntToIP(IPToInt(ip) + diff)
}
