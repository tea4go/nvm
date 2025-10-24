package utility

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Rename 重命名文件或目录，支持跨磁盘操作
func Rename(old, new string) error {
	// 检查源路径和目标路径是否在同一磁盘
	old_drive := filepath.VolumeName(old)
	new_drive := filepath.VolumeName(new)

	// 同一磁盘直接使用系统重命名
	if old_drive == new_drive {
		return os.Rename(old, new)
	}

	// 获取源文件/目录信息
	info, err := os.Stat(old)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// 如果是目录，递归复制
	if info.IsDir() {
		err = copyDir(old, new)
		if err != nil {
			return fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		// 如果是文件，直接复制
		err = copyFile(old, new)
		if err != nil {
			return fmt.Errorf("failed to copy file: %w", err)
		}
	}

	// 删除原始文件/目录
	err = os.RemoveAll(old)
	if err != nil {
		return fmt.Errorf("failed to remove source: %w", err)
	}

	return nil
}

// copyFile 复制单个文件从源路径(old)到目标路径(new)
func copyFile(old, new string) error {
	// 打开源文件
	srcFile, err := os.Open(old)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// 确保目标目录存在
	destDir := filepath.Dir(new)
	err = os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 创建目标文件
	destFile, err := os.Create(new)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// 复制文件内容
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// 复制文件权限
	info, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}
	err = os.Chmod(new, info.Mode())
	if err != nil {
		return fmt.Errorf("failed to set permissions on destination file: %w", err)
	}

	return nil
}

// copyDir 递归复制目录从源路径(old)到目标路径(new)
func copyDir(old, new string) error {
	// 读取源目录内容
	entries, err := os.ReadDir(old)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// 确保目标目录存在
	err = os.MkdirAll(new, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 遍历目录项
	for _, entry := range entries {
		srcPath := filepath.Join(old, entry.Name())
		destPath := filepath.Join(new, entry.Name())

		if entry.IsDir() {
			// 递归复制子目录
			err = copyDir(srcPath, destPath)
			if err != nil {
				return fmt.Errorf("failed to copy subdirectory: %w", err)
			}
		} else {
			// 复制文件
			err = copyFile(srcPath, destPath)
			if err != nil {
				return fmt.Errorf("failed to copy file: %w", err)
			}
		}
	}

	return nil
}
