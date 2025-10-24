// Package author 提供与作者桥接程序交互的功能
// 主要功能包括：
// - 启动和管理author-nvm.exe桥接程序
// - 处理与桥接程序的通信
package author

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coreybutler/go-fsutil"
	"golang.org/x/sys/windows"
)

const (
	// Windows API常量定义
	SWP_NOMOVE     = 0x0002 // 不移动窗口
	SWP_NOZORDER   = 0x0004 // 不改变Z序
	SWP_SHOWWINDOW = 0x0040 // 显示窗口
	SW_HIDE        = 0      // 隐藏窗口(用于主控制台窗口)
)

var (
	modkernel32    = syscall.NewLazyDLL("kernel32.dll")      // kernel32.dll动态链接库
	moduser32      = syscall.NewLazyDLL("user32.dll")        // user32.dll动态链接库
	procGetConsole = modkernel32.NewProc("GetConsoleWindow") // 获取控制台窗口句柄
	procShowWindow = moduser32.NewProc("ShowWindow")         // 显示/隐藏窗口
)

// hideConsole 隐藏主控制台窗口(运行Go应用程序的窗口)
// 内部函数，不导出
func hideConsole() {
	hwnd, _, _ := procGetConsole.Call()
	if hwnd != 0 {
		procShowWindow.Call(hwnd, uintptr(SW_HIDE)) // Hide the main console window
	}
}

// Bridge 启动并管理与author-nvm.exe桥接程序的交互
// 参数:
//
//	args: 传递给桥接程序的参数
//
// 功能:
//  1. 检查桥接程序是否存在
//  2. 验证参数有效性
//  3. 隐藏主控制台窗口
//  4. 启动桥接程序并处理输出
//
// Bridge 函数用于执行与 author-nvm.exe 的桥接通信
func Bridge(args ...string) {
	// 获取当前可执行文件路径并构建桥接程序路径
	exe, _ := os.Executable()
	bridge := filepath.Join(filepath.Dir(exe), "author-nvm.exe")
	// 检查桥接程序是否存在
	if !fsutil.Exists(bridge) {
		fmt.Println("error: author bridge not found")
		os.Exit(1)
	}

	// 验证参数数量 (至少需要2个参数，除非是version命令)
	if len(args) < 2 {
		if !(len(args) == 1 && args[0] == "version") {
			fmt.Printf("error: invalid number of arguments passed to author bridge: %d\n", len(args))
			os.Exit(1)
		}
	}

	// 解析命令和参数
	command := args[0]
	args = args[1:]

	// 隐藏控制台窗口
	hideConsole()

	// 创建并配置子进程
	cmd := exec.Command(bridge, append([]string{command}, args...)...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | windows.CREATE_NO_WINDOW,
	}
	// 创建标准输出和错误输出管道
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	// 启动子进程
	if err := cmd.Start(); err != nil {
		fmt.Println("error starting bridge command:", err)
		os.Exit(1)
	}

	// 异步读取并打印标准输出
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// 异步读取并打印标准错误
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	// 特殊处理升级命令的rollback参数
	if command == "upgrade" {
		for _, arg := range args {
			if strings.Contains(arg, "--rollback") {
				fmt.Println("exiting to rollback nvm.exe...")
				time.Sleep(1 * time.Second)
				os.Exit(0)
			}
		}
	}

	// 等待子进程结束
	if err := cmd.Wait(); err != nil {
		fmt.Println("bridge command finished with error:", err)
	}
}
