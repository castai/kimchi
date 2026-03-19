package gsd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Backup(srcPath string) (string, error) {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read source file: %w", err)
	}

	backupPath := fmt.Sprintf("%s.bak.%d", srcPath, time.Now().Unix())

	if err := os.WriteFile(backupPath, content, 0600); err != nil {
		return "", fmt.Errorf("write backup file: %w", err)
	}

	return backupPath, nil
}

func RestoreBackup(backupPath, originalPath string) error {
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup file: %w", err)
	}

	if err := os.WriteFile(originalPath, content, 0600); err != nil {
		return fmt.Errorf("restore original file: %w", err)
	}

	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("remove backup file: %w", err)
	}

	return nil
}

func BackupFiles(paths []string) ([]string, error) {
	var backupPaths []string
	for _, path := range paths {
		backupPath, err := Backup(path)
		if err != nil {
			for _, bp := range backupPaths {
				os.Remove(bp)
			}
			return nil, fmt.Errorf("backup %s: %w", filepath.Base(path), err)
		}
		backupPaths = append(backupPaths, backupPath)
	}
	return backupPaths, nil
}
