package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"os"
	"os/user"
	"path/filepath"
)

const launchLabel = "com.compozy.daemon"

func (m *Manager) plistPath() string {
	return filepath.Join(m.operatorHome, "Library", "LaunchAgents", launchLabel+".plist")
}

func launchDomain() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", err
	}
	return "gui/" + current.Uid, nil
}

func (m *Manager) installMac(ctx context.Context) (Result, error) {
	domain, err := launchDomain()
	if err != nil {
		return Result{}, err
	}
	target := domain + "/" + launchLabel
	if _, err := run(ctx, "launchctl", "print", target); err == nil {
		if _, err := run(ctx, "launchctl", "bootout", target); err != nil {
			return Result{}, err
		}
	}
	plist, err := m.plist()
	if err != nil {
		return Result{}, err
	}
	path := m.plistPath()
	if err := writeService(path, plist); err != nil {
		return Result{}, err
	}
	if _, err := run(ctx, "launchctl", "bootstrap", domain, path); err != nil {
		return Result{}, err
	}
	return m.statusMac(ctx)
}

func (m *Manager) plist() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString("<plist version=\"1.0\"><dict><key>Label</key><string>" + launchLabel + "</string>")
	b.WriteString("<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>")
	b.WriteString("<key>ProgramArguments</key><array><string>")
	if err := xml.EscapeText(&b, []byte(m.binary)); err != nil {
		return nil, err
	}
	b.WriteString("</string><string>daemon</string><string>start</string><string>--foreground</string></array>")
	b.WriteString("<key>EnvironmentVariables</key><dict><key>COMPOZY_HOME</key><string>")
	if err := xml.EscapeText(&b, []byte(m.home)); err != nil {
		return nil, err
	}
	b.WriteString("</string>")
	if m.cloudURL != "" {
		b.WriteString("<key>COMPOZY_CLOUD_URL</key><string>")
		if err := xml.EscapeText(&b, []byte(m.cloudURL)); err != nil {
			return nil, err
		}
		b.WriteString("</string>")
	}
	b.WriteString("</dict></dict></plist>\n")
	return b.Bytes(), nil
}

func (m *Manager) uninstallMac(ctx context.Context) (Result, error) {
	result, err := m.statusMac(ctx)
	if err != nil || !result.Installed {
		return result, err
	}
	if result.Running {
		domain, err := launchDomain()
		if err != nil {
			return result, err
		}
		if _, err := run(ctx, "launchctl", "bootout", domain+"/"+launchLabel); err != nil {
			return result, err
		}
	}
	err = os.Remove(m.plistPath())
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return Result{Manager: macManager}, err
}

func (m *Manager) statusMac(ctx context.Context) (Result, error) {
	present, err := installed(m.plistPath())
	result := Result{Manager: macManager, Installed: present}
	if err != nil || !present {
		return result, err
	}
	domain, err := launchDomain()
	if err != nil {
		return result, err
	}
	_, err = run(ctx, "launchctl", "print", domain+"/"+launchLabel)
	result.Running = err == nil
	return result, nil
}
