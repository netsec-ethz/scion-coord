package api

import (
	"path/filepath"
	"runtime"
	"time"

	"github.com/netsec-ethz/scion-coord/config"
)

var (
	_, b, _, _      = runtime.Caller(0)
	currentPath     = filepath.Dir(b)
	scionCoordPath  = filepath.Dir(filepath.Dir(currentPath))
	localGenPath    = filepath.Join(scionCoordPath, "python", "local_gen.py")
	TempPath        = filepath.Join(scionCoordPath, "temp")
	scionPath       = filepath.Join(filepath.Dir(scionCoordPath), "scion")
	scionWebPath    = filepath.Join(filepath.Dir(scionCoordPath), "scion-web")
	pythonPath      = filepath.Join(scionPath, "python")
	vagrantPath     = filepath.Join(scionCoordPath, "vagrant")
	PackagePath     = config.PACKAGE_DIRECTORY
	BoxPackagePath  = filepath.Join(PackagePath, "SCIONBox")
	credentialsPath = filepath.Join(scionCoordPath, "credentials")
	CoreCertFile    = filepath.Join(credentialsPath, "ISD1-AS1-V0.crt")
	CoreSigKey      = filepath.Join(credentialsPath, "as-sig.key")
	TrcFile         = filepath.Join(credentialsPath, "ISD1-V0.trc")
	EasyRSAPath     = filepath.Join(PackagePath, "easy-rsa")
	RSAKeyPath      = filepath.Join(EasyRSAPath, "keys")
	CACertPath      = filepath.Join(RSAKeyPath, "ca.crt")
	HeartBeatPeriod = time.Duration(config.HB_PERIOD)
	HeartBeatLimit  = time.Duration(config.HB_LIMIT)
)

const(
	BR_START_PORT = 31046
	MTU = 1472
)
