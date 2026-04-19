package config

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), nil
}

func ParseSSHConfig(path string) ([]Connection, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var conns []Connection
	var current *Connection

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if strings.EqualFold(key, "Host") {
			if val == "*" {
				continue
			}
			if current != nil {
				if current.Port == 0 {
					current.Port = 22
				}
				conns = append(conns, *current)
			}
			current = &Connection{
				Name:                val,
				SyncedFromSSHConfig: true,
			}
		} else if current != nil {
			switch strings.ToLower(key) {
			case "hostname":
				current.Host = val
			case "user":
				current.User = val
			case "port":
				p, err := strconv.Atoi(val)
				if err == nil {
					current.Port = p
				}
			case "identityfile":
				current.IdentityFile = val
			case "proxyjump":
				current.JumpHost = val
			}
		}
	}
	if current != nil {
		if current.Port == 0 {
			current.Port = 22
		}
		conns = append(conns, *current)
	}
	return conns, scanner.Err()
}

func (cfg *HangarConfig) SyncFromSSHConfig(path string) (added, updated int, err error) {
	parsed, err := ParseSSHConfig(path)
	if err != nil {
		return 0, 0, err
	}

	for _, p := range parsed {
		existing, findErr := cfg.FindByName(p.Name)
		if findErr != nil {
			cfg.Connections = append(cfg.Connections, p)
			added++
		} else if existing.SyncedFromSSHConfig {
			changed := existing.Host != p.Host ||
				existing.Port != p.Port ||
				existing.User != p.User ||
				existing.IdentityFile != p.IdentityFile ||
				existing.JumpHost != p.JumpHost
			if changed {
				existing.Host = p.Host
				existing.Port = p.Port
				existing.User = p.User
				existing.IdentityFile = p.IdentityFile
				existing.JumpHost = p.JumpHost
				updated++
			}
		}
	}

	hash, _ := HashFile(path)
	cfg.SSHSync.LastSync = time.Now()
	cfg.SSHSync.LastSSHConfigHash = hash

	return added, updated, nil
}

func (cfg *HangarConfig) NeedsSync(sshConfigPath string) (bool, error) {
	hash, err := HashFile(sshConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return hash != cfg.SSHSync.LastSSHConfigHash, nil
}
