// Package upgrade 提供 Windows 计划任务注册和注销功能
// 主要用于 NVM for Windows 的自动更新检查
package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 计划任务名称常量
const (
	NODE_LTS_SCHEDULE_NAME     = "NVM for Windows Node.js LTS Update Check"     // Node.js LTS 版本更新检查任务名
	NODE_CURRENT_SCHEDULE_NAME = "NVM for Windows Node.js Current Update Check" // Node.js Current 版本更新检查任务名
	NVM4W_SCHEDULE_NAME        = "NVM for Windows Update Check"                 // NVM for Windows 自身更新检查任务名
	AUTHOR_SCHEDULE_NAME       = "NVM for Windows Author Update Check"          // 作者更新检查任务名
)

// Registration 表示需要注册的计划任务类型
type Registration struct {
	LTS     bool // 是否注册 Node.js LTS 版本更新检查
	Current bool // 是否注册 Node.js Current 版本更新检查
	NVM4W   bool // 是否注册 NVM for Windows 自身更新检查
	Author  bool // 是否注册作者更新检查
}

// LoadRegistration 从命令行参数加载注册配置
// 参数:
//
//	args: 命令行参数列表，支持 --lts, --current, --nvm4w, --author
//
// 返回值:
//
//	*Registration: 初始化后的注册配置对象
func LoadRegistration(args ...string) *Registration {
	reg := &Registration{
		LTS:     false,
		Current: false,
		NVM4W:   false,
		Author:  false,
	}

	// 解析命令行参数
	for _, arg := range args {
		arg = strings.ToLower(strings.ReplaceAll(arg, "--", ""))
		switch arg {
		case "lts":
			reg.LTS = true
		case "current":
			reg.Current = true
		case "nvm4w":
			reg.NVM4W = true
		case "author":
			reg.Author = true
		}
	}

	return reg
}

// abortOnError 遇到错误时终止程序并记录错误日志
// 参数:
//
//	err: 需要检查的错误对象
func abortOnError(err error) {
	if err != nil {
		fmt.Println(err)
		os.WriteFile("./error.log", []byte(err.Error()), os.ModePerm)
		os.Exit(1)
	}
}

// logError 记录错误日志但不终止程序
// 参数:
//
//	err: 需要记录的错误对象
func logError(err error) {
	fmt.Println(err)
	if err != nil {
		os.WriteFile("./error.log", []byte(err.Error()), os.ModePerm)
	}
}

// Register 根据配置注册计划任务
// 每小时执行一次对应的更新检查命令
func Register() {
	// 从命令行参数加载注册配置
	reg := LoadRegistration(os.Args[2:]...)
	exe, _ := os.Executable()

	// 根据配置注册不同的计划任务
	if reg.LTS {
		abortOnError(ScheduleTask(NODE_LTS_SCHEDULE_NAME, fmt.Sprintf(`"%s" checkForUpdates lts`, exe), "HOURLY", "00:30"))
	}
	if reg.Current {
		abortOnError(ScheduleTask(NODE_CURRENT_SCHEDULE_NAME, fmt.Sprintf(`"%s" checkForUpdates current`, exe), "HOURLY", "00:25"))
	}
	if reg.NVM4W {
		abortOnError(ScheduleTask(NVM4W_SCHEDULE_NAME, fmt.Sprintf(`"%s" checkForUpdates nvm4w`, exe), "HOURLY", "00:15"))
	}
	if reg.Author {
		abortOnError(ScheduleTask(AUTHOR_SCHEDULE_NAME, fmt.Sprintf(`"%s" checkForUpdates author`, exe), "HOURLY", "00:45"))
	}
}

// Unregister 根据配置注销计划任务
func Unregister() {
	// 从命令行参数加载注册配置
	reg := LoadRegistration(os.Args[2:]...)

	// 根据配置注销不同的计划任务
	if reg.LTS {
		abortOnError(UnscheduleTask(NODE_LTS_SCHEDULE_NAME))
	}
	if reg.Current {
		abortOnError(UnscheduleTask(NODE_CURRENT_SCHEDULE_NAME))
	}
	if reg.NVM4W {
		abortOnError(UnscheduleTask(NVM4W_SCHEDULE_NAME))
	}
	if reg.Author {
		abortOnError(UnscheduleTask(AUTHOR_SCHEDULE_NAME))
	}
}

// ScheduleTask 创建 Windows 计划任务
// 参数:
//
//	name: 任务名称
//	command: 要执行的命令
//	interval: 执行间隔 (MINUTE, HOURLY, DAILY, WEEKLY, MONTHLY, ONCE, ONSTART, ONLOGON, ONIDLE, EVENT)
//	startTime: 可选，任务开始时间，格式为"HH:MM"
//
// 返回值:
//
//	error: 创建任务过程中遇到的错误
func ScheduleTask(name string, command string, interval string, startTime ...string) error {
	// 验证间隔参数有效性
	switch strings.ToUpper(interval) {
	case "MINUTE":
		fallthrough
	case "HOURLY":
		fallthrough
	case "DAILY":
		fallthrough
	case "WEEKLY":
		fallthrough
	case "MONTHLY":
		fallthrough
	case "ONCE":
		fallthrough
	case "ONSTART":
		fallthrough
	case "ONLOGON":
		fallthrough
	case "ONIDLE":
		fallthrough
	case "EVENT":
		interval = strings.ToUpper(interval)
	default:
		return fmt.Errorf("scheduling error: invalid interval %q", interval)
	}

	// 设置默认开始时间
	start := "00:00"
	if len(startTime) > 0 {
		start = startTime[0]
	}

	// 创建临时目录存放批处理脚本
	tmp, err := os.MkdirTemp("", "nvm4w-regitration-*")
	if err != nil {
		return fmt.Errorf("scheduling error: %v", err)
	}
	defer os.RemoveAll(tmp)

	// 生成创建计划任务的批处理脚本
	script := fmt.Sprintf(`
@echo off
set errorlog="error.log"
set output="%s\output.log"
schtasks /create /tn "%s" /tr "cmd.exe /c %s" /sc %s /st %s /F > %%output%% 2>&1
if not errorlevel 0 (
	echo ERROR: Failed to create scheduled task: exit code: %%errorlevel%% >> %%errorlog%%
	type %%output%% >> %%errorlog%%
	exit /b %%errorlevel%%
)
	`, tmp, name, escapeBackslashes(command), strings.ToLower(interval), start)

	// 写入批处理文件
	err = os.WriteFile(filepath.Join(tmp, "schedule.bat"), []byte(script), os.ModePerm)
	if err != nil {
		return fmt.Errorf("scheduling error: %v", err)
	}

	// 执行批处理文件
	cmd := exec.Command(filepath.Join(tmp, "schedule.bat"))

	// 捕获标准输出和标准错误
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scheduling error: %v\n%s", err, out)
	}

	return nil
}

// UnscheduleTask 删除 Windows 计划任务
// 参数:
//
//	name: 要删除的任务名称
//
// 返回值:
//
//	error: 删除任务过程中遇到的错误
func UnscheduleTask(name string) error {
	// 创建临时目录存放批处理脚本
	tmp, err := os.MkdirTemp("", "nvm4w-registration-*")
	if err != nil {
		return fmt.Errorf("scheduling error: %v", err)
	}
	defer os.RemoveAll(tmp)

	// 生成删除计划任务的批处理脚本
	script := fmt.Sprintf(`
@echo off
set errorlog="error.log"
set output="%s\output.log"
schtasks /delete /tn "%s" /f > %%output%% 2>&1
if not errorlevel 0 (
	echo failed to remove scheduled task: exit code: %%errorlevel%% >> %%errorlog%%
	type %%output%% >> %%errorlog%%
	exit /b %%errorlevel%%
)
	`, tmp, name)

	// 写入批处理文件
	err = os.WriteFile(filepath.Join(tmp, "unschedule.bat"), []byte(script), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unscheduling error: %v", err)
	}

	// 执行批处理文件
	cmd := exec.Command(filepath.Join(tmp, "unschedule.bat"))

	// 捕获标准输出和标准错误
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unscheduling error: %v\n%s", err, out)
	}

	return nil
}
