package hosts

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	HostsFile       = "/etc/hosts"
	hostsMarker     = "# Added by OpenHijack"
	hostsDomain     = "api.openai.com"
	hostsBackupName = "hosts.backup"
)

var hostsIPs = []string{"127.0.0.1", "::1"}

type HostsManager struct {
	dataDir string
}

func NewHostsManager(dataDir string) *HostsManager {
	return &HostsManager{dataDir: dataDir}
}

func (hm *HostsManager) BackupPath() string {
	return filepath.Join(hm.dataDir, hostsBackupName)
}

func (hm *HostsManager) RemoveBackup(logf func(string, ...interface{})) error {
	path := hm.BackupPath()
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			logf("hosts 备份文件不存在，跳过移除: %s", path)
			return nil
		}
		return fmt.Errorf("移除 hosts 备份失败: %w", err)
	}
	logf("已移除 hosts 备份: %s", path)
	return nil
}

func (hm *HostsManager) BackupHosts(logf func(string, ...interface{})) error {
	src, err := os.ReadFile(HostsFile)
	if err != nil {
		return fmt.Errorf("读取 hosts 文件失败: %w", err)
	}
	if err := os.MkdirAll(hm.dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	if err := os.WriteFile(hm.BackupPath(), src, 0644); err != nil {
		return fmt.Errorf("备份 hosts 文件失败: %w", err)
	}
	logf("hosts 文件已备份到 %s", hm.BackupPath())
	return nil
}

func (hm *HostsManager) AddEntry(logf func(string, ...interface{})) error {
	if err := hm.BackupHosts(logf); err != nil {
		return err
	}

	content, err := os.ReadFile(HostsFile)
	if err != nil {
		return fmt.Errorf("读取 hosts 文件失败: %w", err)
	}

	if bytes.Contains(content, []byte(hostsMarker)) {
		logf("hosts 文件中已存在 OpenHijack 条目，跳过添加")
		return nil
	}

	var buf strings.Builder
	buf.Write(content)
	if len(content) > 0 && content[len(content)-1] != '\n' {
		buf.WriteByte('\n')
	}
	buf.WriteString(hostsMarker)
	buf.WriteByte('\n')
	for _, ip := range hostsIPs {
		buf.WriteString(ip)
		buf.WriteByte(' ')
		buf.WriteString(hostsDomain)
		buf.WriteByte('\n')
	}

	if err := os.WriteFile(HostsFile, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("写入 hosts 文件失败: %w", err)
	}

	logf("已添加 hosts 条目: %s -> %s", strings.Join(hostsIPs, ", "), hostsDomain)
	return nil
}

func (hm *HostsManager) RemoveEntry(logf func(string, ...interface{})) error {
	content, err := os.ReadFile(HostsFile)
	if err != nil {
		return fmt.Errorf("读取 hosts 文件失败: %w", err)
	}

	if !bytes.Contains(content, []byte(hostsMarker)) {
		logf("hosts 文件中不存在 OpenHijack 条目，跳过移除")
		return nil
	}

	cleaned := removeHostsBlock(string(content))
	if err := os.WriteFile(HostsFile, []byte(cleaned), 0644); err != nil {
		return fmt.Errorf("写入 hosts 文件失败: %w", err)
	}

	logf("已移除 hosts 中的 OpenHijack 条目")
	return nil
}

func (hm *HostsManager) RestoreHosts(logf func(string, ...interface{})) error {
	bp := hm.BackupPath()
	if _, err := os.Stat(bp); os.IsNotExist(err) {
		logf("hosts 备份文件不存在，跳过恢复")
		return nil
	}
	data, err := os.ReadFile(bp)
	if err != nil {
		return fmt.Errorf("读取 hosts 备份失败: %w", err)
	}
	if err := os.WriteFile(HostsFile, data, 0644); err != nil {
		return fmt.Errorf("恢复 hosts 文件失败: %w", err)
	}
	logf("hosts 文件已从备份恢复")
	return nil
}

func removeHostsBlock(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var result []string
	skip := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == hostsMarker {
			skip = true
			continue
		}
		if skip {
			trimmed := strings.TrimSpace(line)
			isEntry := false
			for _, ip := range hostsIPs {
				if trimmed == ip+" "+hostsDomain || trimmed == ip+"\t"+hostsDomain {
					isEntry = true
					break
				}
			}
			if isEntry {
				continue
			}
			skip = false
		}
		result = append(result, line)
	}

	var buf bytes.Buffer
	for i, line := range result {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
	}
	return buf.String()
}
