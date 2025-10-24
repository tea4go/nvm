package utility

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// 调试日志开关
var debug bool = false

// 可执行文件路径
var exe string

// 项目根路径
var path string

const (
	// Windows上启用虚拟终端处理(用于解释ANSI转义码)
	enableVirtualTerminalProcessing = 0x0004
	// 粗体橙色文本
	BOLD = "\033[38;2;255;165;0m"
	// 浅黄色文本
	TEXT = "\033[38;2;255;200;100m"
	// 重置文本样式
	RESET = "\033[0m"
)

// enableANSI 在Windows上启用ANSI转义码支持
func enableANSI() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")
	stdout := syscall.Stdout

	// 获取当前控制台模式
	var mode uint32
	err := syscall.GetConsoleMode(stdout, &mode)
	if err != nil {
		fmt.Println("Error getting console mode:", err)
		return
	}

	// 启用虚拟终端处理
	mode |= enableVirtualTerminalProcessing
	_, _, err = setConsoleMode.Call(uintptr(stdout), uintptr(mode))
	if err != nil && err.Error() != "The operation completed successfully." {
		fmt.Println("Error enabling ANSI:", err)
	}
}

// bold 返回带粗体橙色样式的文本
func bold(text string) string {
	return BOLD + text + RESET
}

// text 返回带浅黄色样式的文本
func text(txt string) string {
	return TEXT + txt + RESET
}

// EnableDebugLogs 启用调试日志并初始化相关配置
func EnableDebugLogs() {
	debug = true
	exe, _ = os.Executable()
	path = filepath.Join(filepath.Dir(exe), "..")
	enableANSI()
}

// DebugLog 打印调试日志(可变参数)
func DebugLog(args ...interface{}) {
	if debug {
		_, file, line, _ := runtime.Caller(1)
		for _, arg := range args {
			fmt.Printf(bold("[DEBUG] %v:%v")+" "+text("%v")+"\n",
				strings.Replace(filepath.ToSlash(file), filepath.ToSlash(path), "..", 1),
				line, arg)
		}
	}
}

// DebugLogf 打印格式化调试日志
func DebugLogf(tpl string, args ...interface{}) {
	if debug {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf(bold("[DEBUG] %v:%v")+" "+text("%v")+"\n",
			strings.Replace(filepath.ToSlash(file), filepath.ToSlash(path), "..", 1),
			line, fmt.Sprintf(tpl, args...))
	}
}

// DebugFn 仅在调试模式下执行函数
func DebugFn(fn func()) {
	if debug {
		fn()
	}
}
