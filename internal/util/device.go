package util

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"sync"
)

var (
	fingerprintOnce sync.Once
	fingerprintVal  string
)

// MachineFingerprint 返回本机稳定指纹：sha256("<GOOS>|<原始机器标识>") 的十六进制。
// 只保留哈希，原始标识不外传。结果在进程内缓存一次。
func MachineFingerprint() string {
	fingerprintOnce.Do(func() {
		sum := sha256.Sum256([]byte(runtime.GOOS + "|" + rawMachineID()))
		fingerprintVal = hex.EncodeToString(sum[:])
	})
	return fingerprintVal
}

// Hostname 返回本机主机名，作为设备默认名称。失败时返回 "unknown"。
func Hostname() string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return "unknown"
}

// rawMachineID 按平台读取稳定机器标识；全部失败时降级为 hostname+username。
func rawMachineID() string {
	switch runtime.GOOS {
	case "linux":
		for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
			if b, err := os.ReadFile(p); err == nil {
				if s := strings.TrimSpace(string(b)); s != "" {
					return s
				}
			}
		}
	case "darwin":
		// ioreg 输出形如：| |   "IOPlatformUUID" = "00000000-0000-..."
		if out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output(); err == nil {
			if v := quotedValue(string(out), "IOPlatformUUID"); v != "" {
				return v
			}
		}
	case "windows":
		// reg query 输出末列为 MachineGuid
		if out, err := exec.Command("reg", "query",
			`HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output(); err == nil {
			fields := strings.Fields(string(out))
			if len(fields) > 0 {
				return fields[len(fields)-1]
			}
		}
	}

	name := ""
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	return Hostname() + "|" + name
}

// quotedValue 从文本中提取 `"key" = "value"` 的 value；未找到返回空串。
func quotedValue(text, key string) string {
	i := strings.Index(text, `"`+key+`"`)
	if i < 0 {
		return ""
	}
	rest := text[i+len(key)+2:]
	eq := strings.Index(rest, "=")
	if eq < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[eq+1:])
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	if end := strings.Index(rest, `"`); end >= 0 {
		return rest[:end]
	}
	return ""
}
