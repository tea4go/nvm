// Package upgrade 提供NVM的升级功能
// 主要功能包括：
// - 检查新版本
// - 下载和验证升级包
// - 执行升级流程
// - 处理升级过程中的错误和回滚
package upgrade

import (
	"archive/zip"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nvm/author"
	"nvm/semver"
	"nvm/utility"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/coreybutler/go-fsutil"
	"github.com/ncruces/zenity"
	"golang.org/x/sys/windows"
)

const (
	UPDATE_URL = "https://api.github.com/repos/coreybutler/nvm-windows/releases/latest" // GitHub API获取最新版本URL
	ALERTS_URL = "https://author.io/nvm4w/feed/alerts"                                  // 警告信息获取URL

	// 终端颜色代码
	yellow = "\033[33m" // 黄色
	reset  = "\033[0m"  // 重置颜色

	// Windows控制台模式
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004     // 启用虚拟终端处理
	FILE_ATTRIBUTE_HIDDEN              = 0x2        // 文件隐藏属性
	CREATE_NEW_CONSOLE                 = 0x00000010 // 为子进程创建新控制台
	DETACHED_PROCESS                   = 0x00000008 // 从父进程分离子进程

	warningIcon = "⚠️" // 警告图标
)

// Notification 表示系统通知的结构体
type Notification struct {
	AppID    string   `json:"app_id"`   // 应用ID
	Title    string   `json:"title"`    // 通知标题
	Message  string   `json:"message"`  // 通知内容
	Icon     string   `json:"icon"`     // 图标类型
	Actions  []Action `json:"actions"`  // 可执行操作列表
	Duration string   `json:"duration"` // 显示时长
	Link     string   `json:"link"`     // 相关链接
}

// Action 表示通知中的可执行操作
type Action struct {
	Type  string `json:"type"`  // 操作类型
	Label string `json:"label"` // 操作显示文本
	URI   string `json:"uri"`   // 操作目标URI
}

// display 发送系统通知
// 参数:
//
//	data: 通知内容
func display(data Notification) {
	data.AppID = "NVM for Windows"
	content, _ := json.Marshal(data)
	go author.Bridge("notify", string(content))
}

// Update 表示可用的更新信息
type Update struct {
	Version         string   `json:"version"`        // 新版本号
	Assets          []string `json:"assets"`         // 附加资源列表
	Warnings        []string `json:"notices"`        // 通用警告信息
	VersionWarnings []string `json:"versionNotices"` // 版本特定警告
	SourceURL       string   `json:"sourceTpl"`      // 更新包下载URL模板
}

// Release 表示GitHub发布的版本信息
type Release struct {
	Version string                   `json:"name"`         // 版本号
	Assets  []map[string]interface{} `json:"assets"`       // 资源列表
	Publish time.Time                `json:"published_at"` // 发布时间
}

// Run 执行升级流程的主函数
// 参数:
//
//	version: 当前版本号
//
// 返回值: 升级过程中遇到的错误
// 功能:
//   - 检查是否需要显示进度UI
//   - 设置信号处理
//   - 启动升级流程
func Run(version string) error {
	show_progress := false
	for _, arg := range os.Args[2:] {
		if strings.ToLower(arg) == "--show-progress-ui" {
			show_progress = true
			break
		}
	}

	status := make(chan Status)

	if !show_progress {
		// Setup signal handling first
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signalChan)

		// Add signal handler
		go func() {
			<-signalChan
			fmt.Println("Installation canceled by user")
			os.Exit(0)
		}()

		go func() {
			for {
				select {
				case s := <-status:
					if s.Warn != "" {
						Warn(s.Warn)
					}
					if s.Err != nil {
						fmt.Println(s.Err)
						os.Exit(1)
					}

					if s.Text != "" {
						fmt.Println(s.Text)
					}

					if s.Done {
						fmt.Println("Upgrade complete")
						return
					}
				}
			}
		}()

		time.Sleep(300 * time.Millisecond)

		return run(version, status)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	var dlg zenity.ProgressDialog
	var exitCode = 0
	var u *Update

	// Display visual progress UI
	go func() {
		defer wg.Done()

		for {
			select {
			case s := <-status:
				if s.Warn != "" {
					display(Notification{
						Message: s.Warn,
						Icon:    "nvm",
					})
				}

				if s.Cancel {
					display(Notification{
						Title:   "Installation Canceled",
						Message: fmt.Sprintf("Installation of NVM for Windows v%s was canceled by the user.", u.Version),
						Icon:    "error",
						Actions: []Action{
							{Type: "protocol", Label: "Install Again", URI: fmt.Sprintf("nvm://launch?action=upgrade&version=%s", u.Version)},
						},
					})

					return
				}

				if s.Err != nil {
					exitCode = 1
					display(Notification{
						Title:   "Installation Error",
						Message: s.Err.Error(),
						Icon:    "error",
					})

					dlg.Text(fmt.Sprintf("error: %v", s.Err))
					dlg.Close()

					// err := restore(status, verbose)
					// if err != nil {
					// 	if show_progress {
					// 		display(notify.Notification{
					// 			Title:   "Rollback Error",
					// 			Message: err.Error(),
					// 			Icon:    "error",
					// 		})
					// 	} else {
					// 		fmt.Printf("rollback error: %v\n", err)
					// 	}
					// }

					return
				}

				if s.Done {
					display(Notification{
						Title:   "Upgrade Complete",
						Message: fmt.Sprintf("Now running version %s.", u.Version),
						Icon:    "success",
					})

					dlg.Text("Upgrade complete")
					time.Sleep(1 * time.Second)

					return
				}

				var empty string
				if s.Text != empty && len(strings.TrimSpace(s.Text)) > 0 {
					dlg.Text(s.Text)
				}
			}
		}
	}()

	// Wait for the prior subroutine to initialize before starting the next dependent thread
	time.Sleep(300 * time.Millisecond)

	// Run the update
	go func() {
		exe, _ := os.Executable()
		winIco := filepath.Join(filepath.Dir(exe), "nvm.ico")
		ico := filepath.Join(filepath.Dir(exe), "download.ico")

		var err error
		u, err = checkForUpdate(UPDATE_URL)
		if err != nil {
			display(Notification{
				Title:   "Update Error",
				Message: err.Error(),
				Icon:    "error",
			})

			status <- Status{Err: fmt.Errorf("error: failed to obtain update data: %v\n", err)}
		}

		var perr error
		dlg, perr = zenity.Progress(
			zenity.Title(fmt.Sprintf("Installing NVM for Windows v%s", u.Version)),
			zenity.Icon(ico),
			zenity.WindowIcon(winIco),
			zenity.AutoClose(),
			zenity.NoCancel(),
			zenity.Pulsate())

		if perr != nil {
			fmt.Println("Failed to create progress dialog")
		}

		go func() {
			for {
				select {
				case <-dlg.Done():
					if err := dlg.Complete(); err == zenity.ErrCanceled {
						status <- Status{Cancel: true}
					}
				}
			}
		}()
		status <- Status{Text: "Validating version..."}

		run(version, status, u)
	}()

	wg.Wait()
	os.Exit(exitCode)

	return nil
}

func run(version string, status chan Status, updateMetadata ...*Update) error {
	args := os.Args[2:]
	colorize := true
	if err := EnableVirtualTerminalProcessing(); err != nil {
		colorize = false
	}

	// Retrieve remote metadata
	var update *Update
	if len(updateMetadata) > 0 {
		update = updateMetadata[0]
	} else {
		var err error
		update, err = checkForUpdate(UPDATE_URL)
		if err != nil {
			return fmt.Errorf("error: failed to obtain update data: %v\n", err)
		}
	}

	for _, warning := range update.Warnings {
		status <- Status{Warn: warning}
	}

	verbose := false
	// rollback := false
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--verbose":
			verbose = true
			// case "rollback":
			// 	rollback = true
		}
	}

	// // Check for a backup
	// if rollback {
	// 	if fsutil.Exists(filepath.Join(".", ".update", "nvm4w-backup.zip")) {
	// 		fmt.Println("restoring NVM4W backup...")
	// 		rbtmp, err := os.MkdirTemp("", "nvm-rollback-*")
	// 		if err != nil {
	// 			fmt.Printf("error: failed to create rollback directory: %v\n", err)
	// 			os.Exit(1)
	// 		}
	// 		defer os.RemoveAll(rbtmp)

	// 		err = unzip(filepath.Join(".", ".update", "nvm4w-backup.zip"), rbtmp)
	// 		if err != nil {
	// 			fmt.Printf("error: failed to extract backup: %v\n", err)
	// 			os.Exit(1)
	// 		}

	// 		// Copy the backup files to the current directory
	// 		err = copyDirContents(rbtmp, ".")
	// 		if err != nil {
	// 			fmt.Printf("error: failed to restore backup files: %v\n", err)
	// 			os.Exit(1)
	// 		}

	// 		// Remove the restoration directory
	// 		os.RemoveAll(filepath.Join(".", ".update"))

	// 		fmt.Println("rollback complete")
	// 		rbcmd := exec.Command("nvm.exe", "version")
	// 		o, err := rbcmd.Output()
	// 		if err != nil {
	// 			fmt.Println("error running nvm.exe:", err)
	// 			os.Exit(1)
	// 		}

	// 		exec.Command("schtasks", "/delete", "/tn", "\"RemoveNVM4WBackup\"", "/f").Run()
	// 		fmt.Printf("rollback to v%s complete\n", string(o))
	// 		os.Exit(0)
	// 	} else {
	// 		fmt.Println("no backup available: backups are only available for 7 days after upgrading")
	// 		os.Exit(0)
	// 	}
	// }

	currentVersion, err := semver.New(version)
	if err != nil {
		return err
	}

	updateVersion, err := semver.New(update.Version)
	if err != nil {
		return err
	}

	if currentVersion.LT(updateVersion) {
		if len(update.VersionWarnings) > 0 {
			if len(update.Warnings) > 0 || len(update.VersionWarnings) > 0 {
				fmt.Println("")
			}
			for _, warning := range update.VersionWarnings {
				status <- Status{Warn: warning}
				Warn(warning, colorize)
			}
			fmt.Println("")
		}
		fmt.Printf("upgrading from v%s-->%s\n", version, highlight(update.Version))
		status <- Status{Text: "downloading..."}
	} else {
		status <- Status{Text: "nvm is up to date", Done: true}
		return nil
	}

	// Make temp directory
	tmp, err := os.MkdirTemp("", "nvm-upgrade-*")
	if err != nil {
		status <- Status{Err: fmt.Errorf("error: failed to create temporary directory: %v\n", err)}
	}
	defer os.RemoveAll(tmp)

	// Download the new app
	source := update.SourceURL
	// source := fmt.Sprintf(update.SourceURL, update.Version)
	// source := fmt.Sprintf(update.SourceURL, "1.1.11") // testing
	body, err := get(source)
	if err != nil {
		status <- Status{Err: fmt.Errorf("error: failed to download new version: %v\n", err)}
	}

	os.WriteFile(filepath.Join(tmp, "assets.zip"), body, os.ModePerm)
	os.Mkdir(filepath.Join(tmp, "assets"), os.ModePerm)

	source = source + ".checksum.txt"
	body, err = get(source)
	if err != nil {
		return fmt.Errorf("error: failed to download checksum: %v\n", err)
	}

	os.WriteFile(filepath.Join(tmp, "assets.zip.checksum.txt"), body, os.ModePerm)

	filePath := filepath.Join(tmp, "assets.zip")                  // path to the file you want to validate
	checksumFile := filepath.Join(tmp, "assets.zip.checksum.txt") // path to the checksum file

	// Step 1: Compute the MD5 checksum of the file
	status <- Status{Text: "verifying checksum..."}
	computedChecksum, err := computeMD5Checksum(filePath)
	if err != nil {
		status <- Status{Err: fmt.Errorf("Error computing checksum: %v", err)}
	}

	// Step 2: Read the checksum from the .checksum.txt file
	storedChecksum, err := readChecksumFromFile(checksumFile)
	if err != nil {
		status <- Status{Err: err}
	}

	// Step 3: Compare the computed checksum with the stored checksum
	if strings.ToLower(computedChecksum) != strings.ToLower(storedChecksum) {
		status <- Status{Err: fmt.Errorf("cannot validate update file (checksum mismatch)")}
	}

	status <- Status{Text: "extracting update..."}
	if err := unzip(filepath.Join(tmp, "assets.zip"), filepath.Join(tmp, "assets")); err != nil {
		status <- Status{Err: err}
	}

	// Get any additional assets
	if len(update.Assets) > 0 {
		status <- Status{Text: fmt.Sprintf("downloading %d additional assets...", len(update.Assets))}
		for _, asset := range update.Assets {
			var assetURL string
			if !strings.HasPrefix(asset, "http") {
				assetURL = update.SourceURL
				// assetURL = fmt.Sprintf(update.SourceURL, asset)
			} else {
				assetURL = asset
			}
			assetBody, err := get(assetURL)
			if err != nil {
				status <- Status{Err: fmt.Errorf("error: failed to download asset: %v\n", err)}
			}

			assetPath := filepath.Join(tmp, "assets", asset)
			os.WriteFile(assetPath, assetBody, os.ModePerm)
		}
	}

	// Debugging
	if verbose {
		tree(tmp, "downloaded files (extracted):")
		nvmtestcmd := exec.Command(filepath.Join(tmp, "assets", "nvm.exe"), "version")
		nvmtestcmd.Stdout = os.Stdout
		nvmtestcmd.Stderr = os.Stderr
		err = nvmtestcmd.Run()
		if err != nil {
			fmt.Println("error running nvm.exe:", err)
		}
	}

	// Backup current version to zip
	status <- Status{Text: "applying update..."}
	currentExe, _ := os.Executable()
	currentPath := filepath.Dir(currentExe)
	bkp, err := os.MkdirTemp("", "nvm-backup-*")
	if err != nil {
		status <- Status{Err: fmt.Errorf("error: failed to create backup directory: %v\n", err)}
	}
	defer os.RemoveAll(bkp)

	err = zipDirectory(currentPath, filepath.Join(bkp, "backup.zip"))
	if err != nil {
		status <- Status{Err: fmt.Errorf("error: failed to create backup: %v\n", err)}
	}

	os.MkdirAll(filepath.Join(currentPath, ".update"), os.ModePerm)
	copyFile(filepath.Join(bkp, "backup.zip"), filepath.Join(currentPath, ".update", "nvm4w-backup.zip"))

	// Copy the new files to the current directory
	// copyFile(currentExe, fmt.Sprintf("%s.%s.bak", currentExe, version))
	copyDirContents(filepath.Join(tmp, "assets"), currentPath)
	copyFile(filepath.Join(tmp, "assets", "nvm.exe"), filepath.Join(currentPath, ".update/nvm.exe"))

	if verbose {
		nvmtestcmd := exec.Command(filepath.Join(currentPath, ".update/nvm.exe"), "version")
		nvmtestcmd.Stdout = os.Stdout
		nvmtestcmd.Stderr = os.Stderr
		err = nvmtestcmd.Run()
		if err != nil {
			status <- Status{Err: err}
		}
	}

	// Debugging
	if verbose {
		tree(currentPath, "final directory contents:")
	}

	// Hide the update directory
	setHidden(filepath.Join(currentPath, ".update"))

	// If an "update.exe" exists, run it
	if fsutil.IsExecutable(filepath.Join(tmp, "assets", "update.exe")) {
		err = copyFile(filepath.Join(tmp, "assets", "update.exe"), filepath.Join(currentPath, ".update", "update.exe"))
		if err != nil {
			status <- Status{Err: fmt.Errorf("error: failed to copy update.exe: %v\n", err)}
			os.Exit(1)
		}
	}

	autoupdate(status)

	return nil
}

// Status 表示升级过程中的状态信息
type Status struct {
	Text   string // 状态文本
	Err    error  // 错误信息
	Done   bool   // 是否完成
	Help   bool   // 是否需要帮助
	Cancel bool   // 是否取消
	Warn   string // 警告信息
}

func (u *Update) Available(sinceVersion string) (string, bool, error) {
	currentVersion, err := semver.New(sinceVersion)
	if err != nil {
		return "", false, err
	}

	updateVersion, err := semver.New(u.Version)
	if err != nil {
		return "", false, err
	}

	if currentVersion.LT(updateVersion) {
		return u.Version, true, nil
	}

	return "", false, nil
}

// Warn 显示警告信息
// 参数:
//
//	msg: 警告内容
//	colorized: 是否使用颜色高亮
func Warn(msg string, colorized ...bool) {
	if len(colorized) > 0 && colorized[0] {
		fmt.Println(warningIcon + "  " + highlight(msg))
	} else {
		fmt.Println(strings.ToUpper(msg))
	}
}

// Get 获取最新的更新信息
// 返回值:
//
//	*Update: 更新信息
//	error: 获取过程中遇到的错误
func Get() (*Update, error) {
	return checkForUpdate(UPDATE_URL)
}

// autoupdate 自动执行更新流程(内部函数)
// 参数:
//
//	status: 状态通知通道
func autoupdate(status chan Status) {
	currentPath, err := os.Executable()
	if err != nil {
		status <- Status{Err: err}
		fmt.Println("error getting updater path:", err)
		os.Exit(1)
	}

	// Create temporary directory for the updater script
	tempDir := filepath.Dir(currentPath) // Use the same temp dir as the new executable
	scriptPath := filepath.Join(tempDir, "updater.bat")

	// Temporary batch file that deletes the directory and the scheduled task
	tmp, err := os.MkdirTemp("", "nvm4w-remove-*")
	if err != nil {
		status <- Status{Err: err}
		fmt.Printf("error creating temporary directory: %v", err)
		os.Exit(1)
	}

	// schedule removal of restoration folder for 30 days from now
	tempBatchFile := filepath.Join(tmp, "remove_backup.bat")
	now := time.Now()
	futureDate := now.AddDate(0, 0, 7)
	formattedDate := futureDate.Format("01/02/2006")
	batchContent := fmt.Sprintf(`
@echo off
schtasks /delete /tn "RemoveNVM4WBackup" /f
rmdir /s /q "%s"
`, escapeBackslashes(filepath.Join(filepath.Dir(currentPath), ".update")))

	// Write the batch file to a temporary location
	err = os.WriteFile(tempBatchFile, []byte(batchContent), os.ModePerm)
	if err != nil {
		status <- Status{Err: err}
		fmt.Printf("error creating temporary batch file: %v", err)
		os.Exit(1)
	}

	updaterScript := fmt.Sprintf(`@echo off
setlocal enabledelayedexpansion

echo ========= Update Script Started ========= >> error.log
echo Started updater script with PID %%1 at %%TIME%% >> error.log
echo Source: %%~2 >> error.log
echo Target: %%~3 >> error.log

:wait
timeout /t 1 /nobreak >nul
tasklist /fi "PID eq %%1" 2>nul | find "%%1" >nul
if not errorlevel 1 (
	echo Waiting for PID %%1 to exit at %%TIME%%... >> error.log
	goto :wait
)

echo ========= Starting Copy Operation ========= >> error.log
echo Checking if source (%%~2) exists... >> error.log
if not exist "%%~2" (
	echo ERROR: Source file does not exist: %%~2 >> error.log
	exit /b 1
)
echo Source file exists >> error.log

del "%%~3" >> error.log

echo Checking if target location is writable... >> error.log
echo Test > "%%~dp3test.txt" 2>>error.log
if errorlevel 1 (
	echo ERROR: Target location is not writable: %%~dp3 >> error.log
	exit /b 1
)
del "%%~dp3test.txt"
echo Target location is writable >> error.log

echo Attempting copy at %%TIME%%... >> error.log
echo Running: copy /y "%%~2" "%%~3" >> error.log
copy /y "%%~2" "%%~3" >> error.log 2>&1
if errorlevel 1 (
	echo ERROR: Copy failed with error level %%errorlevel%% >> error.log
	exit /b %%errorlevel%%
)

echo Verifying copy... >> error.log
if not exist "%%~3" (
	echo ERROR: Target file does not exist after copy: %%~3 >> error.log
	exit /b 1
)

del "%%~2" >> error.log
if exist "%%~2" (
	echo ERROR: Source file still exists after deletion: %%~2 >> error.log
	exit /b 1
)

:: Schedule the task to delete the directory
echo schtasks /create /tn "RemoveNVM4WBackup" /tr "cmd.exe /c %s" /sc once /sd %s /st 12:00 /f >> error.log
schtasks /create /tn "RemoveNVM4WBackup" /tr "cmd.exe /c %s" /sc once /sd %s /st 12:00 /f
if not errorlevel 0 (
	echo ERROR: Failed to create scheduled task: exit code: %%errorlevel%% >> error.log
	exit /b %%errorlevel%%
)

echo Update complete >> error.log

del error.log

del "%%~f0"
start "nvm://launch?action=upgrade_notify"
exit /b 0
`, escapeBackslashes(tempBatchFile), formattedDate, escapeBackslashes(tempBatchFile), formattedDate)

	err = os.WriteFile(scriptPath, []byte(updaterScript), os.ModePerm) // Use standard Windows file permissions
	if err != nil {
		status <- Status{Err: err}
		fmt.Printf("error creating updater script: %v", err)
		os.Exit(1)
	}

	// Start the updater script
	cmd := exec.Command(scriptPath, fmt.Sprintf("%d", os.Getpid()), filepath.Join(tempDir, ".update", "nvm.exe"), currentPath)
	err = cmd.Start()
	if err != nil {
		status <- Status{Err: err}
		fmt.Printf("error starting updater script: %v", err)
		os.Exit(1)
	}

	// Exit the current process (delay for cleanup)
	time.Sleep(300 * time.Millisecond)
	status <- Status{Text: "restarting app...", Done: true}
	time.Sleep(2 * time.Second)
	os.Exit(0)
}

// escapeBackslashes 转义路径中的反斜杠(内部函数)
// 参数:
//
//	path: 原始路径
//
// 返回值: 转义后的路径
func escapeBackslashes(path string) string {
	return strings.Replace(path, "\\", "\\\\", -1)
}

// tree 显示目录树结构(调试用)
// 参数:
//
//	dir: 目录路径
//	title: 可选标题
func tree(dir string, title ...string) {
	if len(title) > 0 {
		fmt.Println("\n" + highlight(title[0]))
	}
	cmd := exec.Command("tree", dir, "/F")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Error executing command:", err)
	}
}

// get 发送HTTP GET请求
// 参数:
//
//	url: 请求URL
//	verbose: 是否显示详细日志
//
// 返回值:
//
//	[]byte: 响应内容
//	error: 请求过程中遇到的错误
func get(url string, verbose ...bool) ([]byte, error) {
	if len(verbose) == 0 || verbose[0] {
		fmt.Printf("  GET %s\n", url)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}
	req.Header.Set("User-Agent", "nvm-windows")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("error: received status code %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// checkForUpdate 检查是否有可用更新
// 参数:
//
//	url: 检查更新的URL
//
// 返回值:
//
//	*Update: 更新信息
//	error: 检查过程中遇到的错误
func checkForUpdate(url string) (*Update, error) {
	u := Update{Assets: []string{}, Warnings: []string{}, VersionWarnings: []string{}}
	r := Release{}

	// Make the HTTP GET request
	utility.DebugLogf("checking for updates at %s", url)
	body, err := get(url, false)
	if err != nil {
		return &u, fmt.Errorf("error: reading response body: %v", err)
	}

	// Parse JSON into the struct
	utility.DebugLogf("Received:\n%s", string(body))
	err = json.Unmarshal(body, &r)
	if err != nil {
		return &u, fmt.Errorf("error: parsing release: %v", err)
	}

	u.Version = r.Version
	utility.DebugLogf("latest version: %s", u.Version)

	// Comment the next line when development is complete
	// u.Version = "2.0.0"
	for _, asset := range r.Assets {
		if value, exists := asset["name"]; exists && value.(string) == "update.exe" {
			u.Assets = append(u.Assets, value.(string))
		}
		if value, exists := asset["name"]; exists && value.(string) == "nvm-noinstall.zip" {
			u.SourceURL = asset["browser_download_url"].(string)
		}
	}

	utility.DebugLogf("source URL: %s", u.SourceURL)
	utility.DebugLogf("assets: %v", u.Assets)

	// Get alerts
	utility.DebugLogf("downloading alerts from %s", ALERTS_URL)
	body, err = get(ALERTS_URL, false)
	if err != nil {
		utility.DebugLogf("alert download error: %v", err)
		return &u, err
	}

	utility.DebugLogf("Received:\n%s", string(body))

	var alerts map[string][]interface{}
	err = json.Unmarshal(body, &alerts)
	if err != nil {
		utility.DebugLogf("alert parsing error: %v", err)
	}

	if value, exists := alerts["all"]; exists {
		for _, warning := range value {
			warn := warning.(map[string]interface{})
			if v, exists := warn["message"]; exists {
				u.Warnings = append(u.Warnings, v.(string))
			}
		}
	}

	if value, exists := alerts[u.Version]; exists {
		utility.DebugLogf("version warnings exist for %v\n%v", u.Version, value)
		for _, warning := range value {
			utility.DebugLogf("warning: %v", warning)
			warn := warning.(map[string]interface{})
			if v, exists := warn["message"]; exists {
				u.VersionWarnings = append(u.VersionWarnings, v.(string))
			}
		}
	}

	utility.DebugLogf("warnings: %v", u.Warnings)
	utility.DebugLogf("version warnings: %v", u.VersionWarnings)

	return &u, nil
}

// EnableVirtualTerminalProcessing 启用Windows虚拟终端处理
// 返回值: 操作过程中遇到的错误
func EnableVirtualTerminalProcessing() error {
	// Get the handle to the standard output
	handle := windows.Stdout

	// Retrieve the current console mode
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return err
	}

	// Enable the virtual terminal processing mode
	mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(handle, mode); err != nil {
		return err
	}

	return nil
}

// highlight 高亮显示消息(使用黄色)
// 参数:
//
//	message: 要显示的消息
//
// 返回值: 高亮后的字符串
func highlight(message string) string {
	return fmt.Sprintf("%s%s%s", yellow, message, reset)
}

// Unzip function extracts a zip file to a specified directory
func unzip(src string, dest string) error {
	// Open the zip archive for reading
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// Iterate over each file in the zip archive
	for _, f := range r.File {
		// Build the path for each file in the destination directory
		fpath := filepath.Join(dest, f.Name)

		// Check if the file is a directory
		if f.FileInfo().IsDir() {
			// Create directory if it doesn't exist
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		// Create directories leading to the file if they don't exist
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		// Open the file in the zip archive
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		// Create the destination file
		outFile, err := os.Create(fpath)
		if err != nil {
			return err
		}
		defer outFile.Close()

		// Copy the file contents from the archive to the destination file
		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

// function to compute the MD5 checksum of a file
func computeMD5Checksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := md5.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	// Return the hex string representation of the MD5 hash
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// function to read the checksum from the .checksum.txt file
func readChecksumFromFile(checksumFile string) (string, error) {
	file, err := os.Open(checksumFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var checksum string
	_, err = fmt.Fscan(file, &checksum)
	if err != nil {
		return "", err
	}

	return checksum, nil
}

func copyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destinationFile.Close()

	// Copy contents from the source file to the destination file
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	// Optionally, copy file permissions (this can be skipped if not needed)
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source file info: %v", err)
	}

	err = os.Chmod(dst, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %v", err)
	}

	return nil
}

// copyDirContents copies all the contents (files and subdirectories) of a source directory to a destination directory.
func copyDirContents(srcDir, dstDir string) error {
	// Ensure destination directory exists
	err := os.MkdirAll(dstDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory %s: %v", dstDir, err)
	}

	// Walk through the source directory recursively
	err = filepath.Walk(srcDir, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing %s: %v", srcPath, err)
		}

		// Construct the corresponding path in the destination directory
		relPath, err := filepath.Rel(srcDir, srcPath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %v", srcPath, err)
		}

		dstPath := filepath.Join(dstDir, relPath)

		// If it's a directory, ensure it's created in the destination
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// If it's a file, copy it
		return copyFile(srcPath, dstPath)
	})

	return err
}

// zipDirectory zips the contents of a directory.
func zipDirectory(sourceDir, outputZip string) error {
	// Create the zip file.
	zipFile, err := os.Create(outputZip)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	// Create a new zip writer.
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through the directory.
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path.
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the directory itself but include subdirectories.
		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			// Add a trailing slash for directories in the zip archive.
			relPath += "/"
		}

		// Create a zip header.
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		if info.IsDir() {
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		// Create a writer for the file in the zip archive.
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If the file is not a directory, copy its contents into the archive.
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// setHidden 设置文件/目录为隐藏属性(Windows系统)
// 参数:
//
//	path: 文件/目录路径
//
// 返回值: 操作过程中遇到的错误
func setHidden(path string) error {
	// Convert the path to a UTF-16 encoded string
	lpFileName, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("failed to encode path: %w", err)
	}

	// Call the Windows API function
	ret, _, err := syscall.NewLazyDLL("kernel32.dll").
		NewProc("SetFileAttributesW").
		Call(
			uintptr(unsafe.Pointer(lpFileName)),
			uintptr(FILE_ATTRIBUTE_HIDDEN),
		)

	// Check the result
	if ret == 0 {
		return fmt.Errorf("failed to set hidden attribute: %w", err)
	}
	return nil
}
