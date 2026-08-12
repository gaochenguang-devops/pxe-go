// Package ipmi 封装 ipmitool 命令行调用，实现主机电源与引导管理。
package ipmi

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"pxe-server/internal/config"
	"pxe-server/internal/util"
)

// Client IPMI 操作客户端。
type Client struct {
	cfg *config.Manager
}

// NewClient 创建 IPMI 客户端。
func NewClient(cfg *config.Manager) *Client {
	return &Client{cfg: cfg}
}

// PowerState 电源状态。
type PowerState struct {
	State string `json:"state"`
}

// powerCmd 构造 ipmitool 电源控制命令基础参数。
func (c *Client) baseArgs(addr, user, pass string) []string {
	return []string{
		"-I", "lanplus",
		"-H", addr,
		"-U", user,
		"-P", pass,
	}
}

// runCommand 执行 ipmitool 命令并返回输出，设置超时。
func (c *Client) runCommand(addr, user, pass string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fullArgs := append(c.baseArgs(addr, user, pass), args...)
	cmd := exec.CommandContext(ctx, "ipmitool", fullArgs...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", errTimeout
	}
	return string(out), err
}

// RunPower 执行电源操作（on/off/cycle/status/reset）。
func (c *Client) RunPower(addr, user, pass, action string) (string, error) {
	return c.runCommand(addr, user, pass, "chassis", "power", action)
}

// PowerOn 开机。
func (c *Client) PowerOn(addr, user, pass string) (string, error) {
	return c.RunPower(addr, user, pass, "on")
}

// PowerOff 硬关机。
func (c *Client) PowerOff(addr, user, pass string) (string, error) {
	return c.RunPower(addr, user, pass, "off")
}

// PowerCycle 硬重启。
func (c *Client) PowerCycle(addr, user, pass string) (string, error) {
	return c.RunPower(addr, user, pass, "cycle")
}

// PowerStatus 查询电源状态。
func (c *Client) PowerStatus(addr, user, pass string) (string, error) {
	return c.RunPower(addr, user, pass, "status")
}

// SetBootDevicePXE 设置下次启动设备为 PXE。
func (c *Client) SetBootDevicePXE(addr, user, pass string) (string, error) {
	return c.runCommand(addr, user, pass, "chassis", "bootdev", "pxe", "--options=efiboot")
}

// SetBootDeviceDisk 设置下次启动设备为本地硬盘。
func (c *Client) SetBootDeviceDisk(addr, user, pass string) (string, error) {
	return c.runCommand(addr, user, pass, "chassis", "bootdev", "disk")
}

// DecryptPass 解密主机 IPMI 密码。
func DecryptPass(cipher string, cfg *config.Manager) string {
	return util.DecryptPassword(cipher, cfg.SeedKey())
}

// EncryptPass 加密主机 IPMI 密码。
func EncryptPass(plain string, cfg *config.Manager) string {
	return util.EncryptPassword(plain, cfg.SeedKey())
}

// errTimeout 超时错误。
var errTimeout = errIPMI("ipmitool timeout")

type errIPMI string

func (e errIPMI) Error() string { return string(e) }

// NormalizeAction 标准化电源动作字符串。
func NormalizeAction(action string) string {
	switch strings.ToLower(action) {
	case "on", "off", "cycle", "status", "reset":
		return strings.ToLower(action)
	default:
		return ""
	}
}
